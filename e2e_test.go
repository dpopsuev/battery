package battery_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/battery/cache"
	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/middleware"
	"github.com/dpopsuev/battery/observer"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/battery/typed"
	"github.com/dpopsuev/battery/workbench"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestE2E_StubPipeline proves the full architecture composes with stubs:
// Tool → StubExecutor → (future: Envelope → Clearance)
// This skeleton runs before any real implementation exists.
func TestE2E_StubPipeline(t *testing.T) {
	t.Parallel()

	// 1. Create stub tools.
	readTool := testkit.NewStubTool("read", "Read a file")
	readTool.Result = "file contents here"
	writeTool := testkit.NewStubTool("write", "Write a file")
	writeTool.Result = "written"

	// 2. Create executor with tools.
	executor := testkit.NewStubExecutor(readTool, writeTool)

	// 3. Execute a tool call — simulates what an LLM agent does.
	ctx := context.Background()
	input := json.RawMessage(`{"path": "/main.go"}`)

	result, err := executor.Execute(ctx, "read", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "file contents here" {
		t.Errorf("read result = %q, want file contents here", result.Text())
	}

	// 4. Verify the call was recorded.
	if len(executor.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(executor.Calls))
	}
	if executor.Calls[0].Name != "read" {
		t.Errorf("call name = %q, want read", executor.Calls[0].Name)
	}

	// 5. Execute unknown tool — should fail.
	_, err = executor.Execute(ctx, "delete", nil)
	if !errors.Is(err, tool.ErrNotFound) {
		t.Errorf("expected ErrNotFound for unknown tool, got %v", err)
	}

	// 6. Verify Names() returns sorted tool names.
	names := executor.Names()
	if len(names) != 2 || names[0] != "read" || names[1] != "write" {
		t.Errorf("Names() = %v, want [read write]", names)
	}

	// 7. Verify All() returns all tools.
	all := executor.All()
	if len(all) != 2 {
		t.Errorf("All() = %d, want 2", len(all))
	}
}

// TestE2E_GateEnrichRecord proves the middleware stubs compose
// even before Envelope is implemented.
func TestE2E_GateEnrichRecord(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Gate: allows the call.
	gate := &testkit.StubGate{Allow: true}
	v, err := gate.Check(ctx, "read", nil)
	if err != nil || !v.Allowed {
		t.Fatalf("gate should allow, got %v %v", v, err)
	}

	// Enricher: adds context.
	enricher := &testkit.StubEnricher{Result: "symbols: 42 loaded"}
	enrichment, err := enricher.Enrich(ctx, "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if enrichment != "symbols: 42 loaded" {
		t.Errorf("enrichment = %q", enrichment)
	}

	// Executor: runs the tool.
	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))
	result, err := executor.Execute(ctx, "read", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Recorder: records the execution.
	recorder := &testkit.StubRecorder{}
	recorder.Record(ctx, "read", nil, result, nil, 0) // result is already tool.Result

	// Verify all stubs recorded their calls.
	if gate.Calls != 1 {
		t.Errorf("gate.Calls = %d, want 1", gate.Calls)
	}
	if enricher.Calls != 1 {
		t.Errorf("enricher.Calls = %d, want 1", enricher.Calls)
	}
	if len(recorder.Records) != 1 {
		t.Errorf("recorder.Records = %d, want 1", len(recorder.Records))
	}
}

// TestE2E_MCPClientServerRoundTrip proves the full MCP stack:
// mcpserver.Server builds a server → mcp.MCPAdapter connects as client →
// tools discovered → tool.Execute routes through MCP → result returned.
// This is how implementors and consumers will use Battery.
func TestE2E_MCPClientServerRoundTrip(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Build a server with mcpserver framework.
	srv := battmcp.NewServer("test-instrument", "v1.0.0").
		WithInstructions("Test instrument for E2E").
		Tool(server.ToolMeta{
			Name:        "analyze",
			Description: "Analyze code quality",
			Keywords:    []string{"code", "quality"},
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var args struct {
				Path string `json:"path"`
			}
			json.Unmarshal(input, &args)
			return tool.TextResult(`{"score":95,"path":"` + args.Path + `"}`), nil
		}).
		Tool(server.ToolMeta{
			Name:        "lint",
			Description: "Run linter",
		}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult(`{"issues":0}`), nil
		})

	// 2. Connect via in-memory transport.
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	// 3. MCPAdapter discovers and registers tools.
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "instrument", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// 4. Verify tools are discoverable with prefixed names.
	names := registry.Names()
	if len(names) != 2 {
		t.Fatalf("discovered %d tools, want 2: %v", len(names), names)
	}
	if names[0] != "instrument.analyze" || names[1] != "instrument.lint" {
		t.Errorf("names = %v", names)
	}

	// 5. Execute through the registry — routes through MCP.
	result, err := registry.Execute(ctx, "instrument.analyze", json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var parsed struct {
		Score int    `json:"score"`
		Path  string `json:"path"`
	}
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %q)", err, result.Text())
	}
	if parsed.Score != 95 || parsed.Path != "main.go" {
		t.Errorf("result = %+v", parsed)
	}

	// 6. Clearance filtering works on MCP-backed tools.
	cleared := tool.NewClearance(registry, []string{"instrument.lint"})
	if len(cleared.Names()) != 1 {
		t.Errorf("clearance: %d tools visible, want 1", len(cleared.Names()))
	}
	_, err = cleared.Execute(ctx, "instrument.analyze", nil)
	if err == nil {
		t.Error("clearance should block instrument.analyze")
	}

	// 7. Cleanup.
	if err := adapter.UnregisterMCP("instrument"); err != nil {
		t.Fatalf("UnregisterMCP: %v", err)
	}
	if len(registry.Names()) != 0 {
		t.Errorf("after unregister: %v", registry.Names())
	}
}

// TestE2E_ObserverCacheRoundTrip proves the observer Hook and Cache compose
// with the MCP stack: tool call → Hook fires → result cached → evict → miss.
func TestE2E_ObserverCacheRoundTrip(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Build MCP server with a tool.
	srv := battmcp.NewServer("data-svc", "v1.0.0").
		Tool(server.ToolMeta{Name: "fetch", Description: "Fetch data"}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var args struct {
				ID string `json:"id"`
			}
			json.Unmarshal(input, &args)
			return tool.TextResult(`{"result":"data-` + args.ID + `"}`), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	// 2. Connect via MCPAdapter.
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "data", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// 3. Wire observer Hook to Ring.
	ring := observer.NewRing(100)
	hook := observer.NewRingHook(ring)

	// 4. Wire MemCache.
	mc := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 100})

	// 5. Execute tool, fire hook, cache result.
	input := json.RawMessage(`{"id":"42"}`)
	hook.OnToolCall(ctx, "data.fetch", input)
	start := time.Now()
	result, err := registry.Execute(ctx, "data.fetch", input)
	elapsed := time.Since(start)
	hook.OnToolResult(ctx, "data.fetch", result, err, elapsed)

	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Cache the result.
	if err := mc.Set(ctx, "data", "fetch:42", []byte(result.Text()), 0); err != nil {
		t.Fatalf("Cache Set: %v", err)
	}

	// 6. Verify Hook recorded events.
	events := ring.Last(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Action != "call" || events[0].Tool != "data.fetch" {
		t.Errorf("first event: %+v", events[0])
	}
	if events[1].Action != "call_done" || events[1].Tool != "data.fetch" {
		t.Errorf("second event: %+v", events[1])
	}

	// 7. Cache hit.
	cached, ok, err := mc.Get(ctx, "data", "fetch:42")
	if err != nil || !ok {
		t.Fatalf("expected cache hit, got ok=%v err=%v", ok, err)
	}
	if string(cached) != result.Text() {
		t.Errorf("cached = %q, want %q", cached, result.Text())
	}

	// 8. Evict namespace, verify miss.
	if err := mc.EvictNamespace(ctx, "data"); err != nil {
		t.Fatalf("EvictNamespace: %v", err)
	}
	_, ok, _ = mc.Get(ctx, "data", "fetch:42")
	if ok {
		t.Error("expected cache miss after eviction")
	}
}

// TestE2E_PolicyEnforcement proves policy stubs compose with executor.
func TestE2E_PolicyEnforcement(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	enforcer := &testkit.StubEnforcer{}
	token := testkit.AllowAllToken()

	// Check passes with nil error (allow all).
	err := enforcer.Check(ctx, token, "read", nil)
	if err != nil {
		t.Fatal(err)
	}

	// Set enforcer to deny.
	enforcer.Err = tool.ErrNotFound
	err = enforcer.Check(ctx, token, "write", nil)
	if !errors.Is(err, tool.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

// TestE2E_ToolMetadata proves metadata, availability gating, and
// MaxResultSize truncation compose end-to-end.
func TestE2E_ToolMetadata(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. Tool with metadata (MaxResultSize=20).
	metaTool := &testkit.StubMetadataTool{
		StubTool: *testkit.NewStubTool("verbose", "produces lots of output"),
		Meta:     tool.Metadata{MaxResultSize: 20, Capabilities: []string{"read"}},
	}
	metaTool.Result = "0123456789abcdefghijklmnopqrstuvwxyz" // 36 bytes

	// 2. Tool with dynamic availability (starts available, then disabled).
	dynTool := &testkit.StubAvailableTool{
		StubTool:    *testkit.NewStubTool("dynamic", "can be toggled"),
		IsAvailable: true,
	}
	dynTool.Result = "dynamic-result"

	// 3. Plain tool (no optional interfaces).
	plain := testkit.NewStubTool("plain", "always available")

	// 4. Register all in registry.
	reg := tool.NewRegistry()
	reg.Register(metaTool)
	reg.Register(dynTool)
	reg.Register(plain)

	// 5. Verify all three visible.
	if len(reg.Names()) != 3 {
		t.Fatalf("expected 3 tools, got %v", reg.Names())
	}

	// 6. ToolMetadata is accessible via type assertion.
	for _, tt := range reg.All() {
		if tm, ok := tt.(tool.ToolMetadata); ok {
			meta := tm.Metadata()
			if meta.MaxResultSize != 20 {
				t.Errorf("MaxResultSize = %d, want 20", meta.MaxResultSize)
			}
			if len(meta.Capabilities) != 1 || meta.Capabilities[0] != "read" {
				t.Errorf("Capabilities = %v", meta.Capabilities)
			}
		}
	}

	// 7. Disable dynamic tool.
	dynTool.IsAvailable = false

	names := reg.Names()
	for _, n := range names {
		if n == "dynamic" {
			t.Error("dynamic tool should not appear when unavailable")
		}
	}
	if len(names) != 2 {
		t.Errorf("expected 2 tools after disabling dynamic, got %v", names)
	}

	// 8. Execute unavailable tool returns ErrNotFound.
	_, err := reg.Execute(ctx, "dynamic", nil)
	if !errors.Is(err, tool.ErrNotFound) {
		t.Errorf("expected ErrNotFound for unavailable, got %v", err)
	}

	// 9. Re-enable.
	dynTool.IsAvailable = true
	_, err = reg.Execute(ctx, "dynamic", nil)
	if err != nil {
		t.Errorf("execute re-enabled dynamic: %v", err)
	}
}

// TestE2E_TypedToolOnMCPServer proves a TypedTool with auto-derived schema
// can be registered on an MCP server and called by a client with typed args.
func TestE2E_TypedToolOnMCPServer(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type analyzeInput struct {
		Path  string `json:"path"`
		Depth int    `json:"depth,omitempty"`
	}
	type analyzeOutput struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}

	// 1. Create TypedTool[In, Out] with auto-derived schemas.
	tt := typed.New("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (analyzeOutput, error) {
		return analyzeOutput(in), nil
	})

	// 2. Register on MCP server using the tool's own schema.
	srv := battmcp.NewServer("typed-svc", "v1.0.0").
		ToolWithSchema(
			server.ToolMeta{Name: tt.Name(), Description: tt.Description()},
			tt.InputSchema(),
			func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tt.Execute(ctx, input)
			},
		)

	// 3. Connect client.
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "typed", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// 4. Verify tool discovered with schema.
	names := registry.Names()
	if len(names) != 1 || names[0] != "typed.analyze" {
		t.Fatalf("names = %v, want [typed.analyze]", names)
	}

	// 5. Execute with typed input.
	result, err := registry.Execute(ctx, "typed.analyze", json.RawMessage(`{"path":"main.go","depth":3}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var parsed struct {
		Path  string `json:"path"`
		Depth int    `json:"depth"`
	}
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %q)", err, result.Text())
	}
	if parsed.Path != "main.go" || parsed.Depth != 3 {
		t.Errorf("result = %+v, want {main.go 3}", parsed)
	}
}

// TestE2E_Workbench proves the Workbench composes MCP-backed tools and
// builtin tools with pipes and conditional swaps.
func TestE2E_Workbench(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Build MCP server with a tool.
	srv := battmcp.NewServer("svc", "v1.0.0").
		Tool(server.ToolMeta{Name: "analyze", Description: "Analyze"}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var args struct {
				Path string `json:"path"`
			}
			json.Unmarshal(input, &args)
			return tool.TextResult("analyzed:" + args.Path), nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	// 2. Connect via MCPAdapter.
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "svc", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// 3. Create a builtin tool.
	formatter := testkit.NewStubTool("format", "Format output")
	formatter.Result = "formatted"

	// 4. Compose Workbench: mount MCP tools + craft builtin.
	wb := workbench.New().
		Mount(registry).
		Craft(formatter)

	// 5. Verify all tools visible.
	names := wb.Names()
	if len(names) != 2 {
		t.Fatalf("Workbench names = %v, want 2 tools", names)
	}

	// 6. Execute MCP tool through workbench.
	result, err := wb.Execute(ctx, "svc.analyze", json.RawMessage(`{"path":"main.go"}`))
	if err != nil {
		t.Fatalf("Execute svc.analyze: %v", err)
	}
	if result.Text() != "analyzed:main.go" {
		t.Errorf("result = %q", result.Text())
	}

	// 7. Execute builtin through workbench.
	result, err = wb.Execute(ctx, "format", nil)
	if err != nil {
		t.Fatalf("Execute format: %v", err)
	}
	if result.Text() != "formatted" {
		t.Errorf("format result = %q", result.Text())
	}

	// 8. Pipe: chain analyze → format.
	wb.Pipe("analyze-then-format", "svc.analyze", "format")
	result, err = wb.Execute(ctx, "analyze-then-format", json.RawMessage(`{"path":"test.go"}`))
	if err != nil {
		t.Fatalf("Execute pipe: %v", err)
	}
	// Final result is format tool's output.
	if result.Text() != "formatted" {
		t.Errorf("pipe result = %q, want formatted", result.Text())
	}

	// 9. Swap: conditional tool selection.
	fastRead := testkit.NewStubTool("read", "fast reader")
	fastRead.Result = "fast"
	slowRead := testkit.NewStubTool("read", "slow reader")
	slowRead.Result = "slow"

	usesFast := true
	wb.Swap(workbench.SwapRule{
		Name:      "read",
		Predicate: func() bool { return usesFast },
		Primary:   fastRead,
		Fallback:  slowRead,
	})

	result, err = wb.Execute(ctx, "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "fast" {
		t.Errorf("swap primary: got %q, want fast", result.Text())
	}

	usesFast = false
	result, err = wb.Execute(ctx, "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "slow" {
		t.Errorf("swap fallback: got %q, want slow", result.Text())
	}
}

// TestE2E_GaugeBasic proves the Gauged optional interface works
// at the tool level — type-assert, call LastMeasurement.
func TestE2E_GaugeBasic(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	gauged := &testkit.StubGaugedTool{
		StubTool: *testkit.NewStubTool("analyze", "Analyze code"),
		Measurements: []tool.Measurement{
			{Name: "tokens_in", Value: 42, Unit: "tokens"},
			{Name: "files_scanned", Value: 7, Unit: "count"},
		},
	}
	gauged.Result = "analysis complete"

	reg := tool.NewRegistry()
	reg.Register(gauged)

	result, err := reg.Execute(ctx, "analyze", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "analysis complete" {
		t.Errorf("result = %q", result.Text())
	}

	// Type-assert Gauged on the tool.
	t2, _ := reg.Get("analyze")
	g, ok := t2.(tool.Gauged)
	if !ok {
		t.Fatal("expected tool to implement Gauged")
	}
	ms := g.LastMeasurement()
	if len(ms) != 2 {
		t.Fatalf("expected 2 measurements, got %d", len(ms))
	}
	if ms[0].Name != "tokens_in" || ms[0].Value != 42 {
		t.Errorf("measurement[0] = %+v", ms[0])
	}

	// Plain tool does not implement Gauged.
	plain := testkit.NewStubTool("plain", "")
	reg.Register(plain)
	t3, _ := reg.Get("plain")
	if _, ok := t3.(tool.Gauged); ok {
		t.Error("plain tool should not implement Gauged")
	}
}

// TestE2E_GaugeFullChain proves the full gauge pipeline:
// StubGaugedTool → Envelope + GaugeFunc(RingHook) → gauge event in Ring.
func TestE2E_GaugeFullChain(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. Gauged tool.
	gauged := &testkit.StubGaugedTool{
		StubTool: *testkit.NewStubTool("analyze", "Analyze"),
		Measurements: []tool.Measurement{
			{Name: "tokens", Value: 42, Unit: "tok"},
		},
	}
	gauged.Result = "done"

	// 2. Non-gauged tool.
	plain := testkit.NewStubTool("lint", "Lint")
	plain.Result = "clean"

	executor := testkit.NewStubExecutor(gauged, plain)

	// 3. Observer ring + hook.
	ring := observer.NewRing(100)
	hook := observer.NewRingHook(ring)

	// 4. Build Envelope with GaugeFunc wired to hook.
	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithGaugeFunc(hook.OnGaugeMeasurement).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// 5. Execute gauged tool.
	result, err := env.Execute(ctx, "analyze", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "done" {
		t.Errorf("result = %q", result.Text())
	}

	// 6. Verify gauge event in Ring.
	events := ring.Last(10)
	var gaugeEvent *observer.Event
	for i := range events {
		if events[i].Action == "gauge" {
			gaugeEvent = &events[i]
			break
		}
	}
	if gaugeEvent == nil {
		t.Fatal("expected gauge event in ring")
	}
	if gaugeEvent.Tool != "analyze" {
		t.Errorf("gauge event tool = %q", gaugeEvent.Tool)
	}
	if gaugeEvent.Metadata["tokens"] != "42 tok" {
		t.Errorf("gauge metadata = %v", gaugeEvent.Metadata)
	}

	// 7. Execute non-gauged tool — no gauge event.
	beforeCount := len(ring.Last(100))
	_, err = env.Execute(ctx, "lint", nil)
	if err != nil {
		t.Fatal(err)
	}
	afterEvents := ring.Last(100)
	gaugeCount := 0
	for _, e := range afterEvents[beforeCount:] {
		if e.Action == "gauge" {
			gaugeCount++
		}
	}
	if gaugeCount != 0 {
		t.Errorf("expected no gauge event for non-gauged tool, got %d", gaugeCount)
	}
}

// TestE2E_MCPRoundTripFidelity proves that multi-content results survive
// the Battery round-trip: MCP server → MCPAdapter → tool.Result → mcpserver → client.
func TestE2E_MCPRoundTripFidelity(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Server returns a multi-content result with structured content.
	srv := battmcp.NewServer("multi-svc", "v1.0.0").
		Tool(server.ToolMeta{Name: "rich", Description: "Rich output"}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			r, _ := tool.StructuredResult(map[string]any{"score": 95})
			// Add an image content block alongside the text fallback.
			r.Content = append(r.Content, tool.ImageContent{MIMEType: "image/png", Data: []byte("fakepng")})
			return r, nil
		})

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	// 2. Client receives via MCPAdapter.
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "multi", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	result, err := registry.Execute(ctx, "multi.rich", nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// 3. Verify multi-content preserved.
	textCount, imageCount := 0, 0
	for _, c := range result.Content {
		switch v := c.(type) {
		case tool.TextContent:
			textCount++
			_ = v
		case tool.ImageContent:
			imageCount++
			if v.MIMEType != "image/png" {
				t.Errorf("image MIME = %q", v.MIMEType)
			}
			if string(v.Data) != "fakepng" {
				t.Errorf("image data = %q", v.Data)
			}
		}
	}
	if textCount < 1 {
		t.Error("expected at least 1 TextContent block")
	}
	if imageCount != 1 {
		t.Errorf("expected 1 ImageContent, got %d", imageCount)
	}

	// 4. Verify structured content preserved.
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent is nil — lost in round-trip")
	}
}

// TestE2E_TypedToolOutputSchema proves TypedTool[In, Out] registers output schema
// and returns StructuredContent through the MCP round-trip.
func TestE2E_TypedToolOutputSchema(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	type searchInput struct {
		Query string `json:"query"`
	}
	type searchOutput struct {
		Matches int    `json:"matches"`
		Query   string `json:"query"`
	}

	tt := typed.New("search", "Search", func(_ context.Context, in searchInput) (searchOutput, error) {
		return searchOutput{Matches: 5, Query: in.Query}, nil
	})

	// Verify OutputSchema is derived.
	if len(tt.OutputSchema()) == 0 {
		t.Fatal("OutputSchema is empty")
	}

	// Register on MCP server and call through client.
	srv := battmcp.NewServer("search-svc", "v1.0.0").
		ToolWithSchema(
			server.ToolMeta{Name: tt.Name(), Description: tt.Description()},
			tt.InputSchema(),
			func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				return tt.Execute(ctx, input)
			},
		)

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "search", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	result, err := registry.Execute(ctx, "search.search", json.RawMessage(`{"query":"battery"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	// Verify StructuredContent round-tripped.
	if result.StructuredContent == nil {
		t.Fatal("StructuredContent nil after round-trip")
	}

	var parsed searchOutput
	if err := json.Unmarshal([]byte(result.Text()), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed.Matches != 5 || parsed.Query != "battery" {
		t.Errorf("parsed = %+v", parsed)
	}
}

// TestE2E_CachePipeline proves the Envelope cache step:
// same args → cache hit, different args → miss, Gauge skipped on hit.
func TestE2E_CachePipeline(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// 1. Cacheable + Gauged tool.
	callCount := 0
	cacheable := &testkit.StubCacheableTool{
		StubTool: *testkit.NewStubTool("compute", "Expensive computation"),
		KeyFn: func(input json.RawMessage) (string, bool) {
			return "key:" + string(input), true
		},
	}
	cacheable.Result = "computed"

	// Wrap to count actual executions.
	wrappedTools := testkit.NewStubExecutor(cacheable)

	// 2. Observer + cache.
	ring := observer.NewRing(100)
	hook := observer.NewRingHook(ring)
	mc := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 100})

	// 3. Build Envelope with cache.
	env, err := middleware.NewBuilder(wrappedTools).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithCache(mc, hook, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	input := json.RawMessage(`{"x":1}`)

	// 4. First call — cache miss, tool executes.
	result, err := env.Execute(ctx, "compute", input)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "computed" {
		t.Errorf("result = %q", result.Text())
	}
	callCount = len(wrappedTools.Calls)
	if callCount != 1 {
		t.Fatalf("expected 1 execution, got %d", callCount)
	}

	// Verify cache miss event.
	missEvents := 0
	for _, e := range ring.Last(100) {
		if e.Action == "cache_miss" {
			missEvents++
		}
	}
	if missEvents != 1 {
		t.Errorf("expected 1 cache_miss event, got %d", missEvents)
	}

	// 5. Second call same args — cache hit, tool does NOT execute.
	result2, err := env.Execute(ctx, "compute", input)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Text() != "computed" {
		t.Errorf("cached result = %q", result2.Text())
	}
	if len(wrappedTools.Calls) != 1 {
		t.Errorf("tool should not have been called again, calls = %d", len(wrappedTools.Calls))
	}

	// Verify cache hit event.
	hitEvents := 0
	for _, e := range ring.Last(100) {
		if e.Action == "cache_hit" {
			hitEvents++
		}
	}
	if hitEvents != 1 {
		t.Errorf("expected 1 cache_hit event, got %d", hitEvents)
	}

	// 6. Third call different args — cache miss, tool executes again.
	_, err = env.Execute(ctx, "compute", json.RawMessage(`{"x":2}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(wrappedTools.Calls) != 2 {
		t.Errorf("expected 2 executions after different args, got %d", len(wrappedTools.Calls))
	}
}
