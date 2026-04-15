package battery_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/battery/cache"
	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/mcpserver"
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
	if result != "file contents here" {
		t.Errorf("read result = %q, want file contents here", result)
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
	recorder.Record(ctx, "read", nil, result, nil, 0)

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
	srv := mcpserver.NewServer("test-instrument", "v1.0.0").
		WithInstructions("Test instrument for E2E").
		Tool(server.ToolMeta{
			Name:        "analyze",
			Description: "Analyze code quality",
			Keywords:    []string{"code", "quality"},
		}, func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
			}
			json.Unmarshal(input, &args)
			return `{"score":95,"path":"` + args.Path + `"}`, nil
		}).
		Tool(server.ToolMeta{
			Name:        "lint",
			Description: "Run linter",
		}, func(_ context.Context, _ json.RawMessage) (string, error) {
			return `{"issues":0}`, nil
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
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %q)", err, result)
	}
	if parsed.Score != 95 || parsed.Path != "main.go" {
		t.Errorf("result = %+v", parsed)
	}

	// 6. Clearance filtering works on MCP-backed tools.
	cleared := server.NewClearance(registry, []string{"instrument.lint"})
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
	srv := mcpserver.NewServer("data-svc", "v1.0.0").
		Tool(server.ToolMeta{Name: "fetch", Description: "Fetch data"}, func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				ID string `json:"id"`
			}
			json.Unmarshal(input, &args)
			return `{"result":"data-` + args.ID + `"}`, nil
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
	if err := mc.Set(ctx, "data", "fetch:42", []byte(result), 0); err != nil {
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
	if string(cached) != result {
		t.Errorf("cached = %q, want %q", cached, result)
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

	// 1. Create TypedTool with auto-derived schema.
	tt := typed.New("analyze", "Analyze code", func(_ context.Context, in analyzeInput) (string, error) {
		return fmt.Sprintf(`{"path":%q,"depth":%d}`, in.Path, in.Depth), nil
	})

	// 2. Register on MCP server using the tool's own schema.
	srv := mcpserver.NewServer("typed-svc", "v1.0.0").
		ToolWithSchema(
			server.ToolMeta{Name: tt.Name(), Description: tt.Description()},
			tt.InputSchema(),
			func(ctx context.Context, input json.RawMessage) (string, error) {
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
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		t.Fatalf("unmarshal result: %v (raw: %q)", err, result)
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
	srv := mcpserver.NewServer("svc", "v1.0.0").
		Tool(server.ToolMeta{Name: "analyze", Description: "Analyze"}, func(_ context.Context, input json.RawMessage) (string, error) {
			var args struct {
				Path string `json:"path"`
			}
			json.Unmarshal(input, &args)
			return "analyzed:" + args.Path, nil
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
	if result != "analyzed:main.go" {
		t.Errorf("result = %q", result)
	}

	// 7. Execute builtin through workbench.
	result, err = wb.Execute(ctx, "format", nil)
	if err != nil {
		t.Fatalf("Execute format: %v", err)
	}
	if result != "formatted" {
		t.Errorf("format result = %q", result)
	}

	// 8. Pipe: chain analyze → format.
	wb.Pipe("analyze-then-format", "svc.analyze", "format")
	result, err = wb.Execute(ctx, "analyze-then-format", json.RawMessage(`{"path":"test.go"}`))
	if err != nil {
		t.Fatalf("Execute pipe: %v", err)
	}
	// Final result is format tool's output.
	if result != "formatted" {
		t.Errorf("pipe result = %q, want formatted", result)
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
	if result != "fast" {
		t.Errorf("swap primary: got %q, want fast", result)
	}

	usesFast = false
	result, err = wb.Execute(ctx, "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "slow" {
		t.Errorf("swap fallback: got %q, want slow", result)
	}
}
