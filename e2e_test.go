package battery_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
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
	if err != tool.ErrNotFound {
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
	if err != tool.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
