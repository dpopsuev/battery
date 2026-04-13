package battery_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/mcpserver"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
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
