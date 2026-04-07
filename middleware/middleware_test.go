package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/middleware"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

func TestEnvelope_GateBlocks(t *testing.T) {
	t.Parallel()
	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))
	gate := testkit.NewStubSecurityGate(false, "denied by policy")

	env, err := middleware.NewBuilder(executor).WithGate(gate).Build()
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.Execute(context.Background(), "read", nil)
	if !errors.Is(err, middleware.ErrToolDenied) {
		t.Errorf("expected ErrToolDenied, got %v", err)
	}
}

func TestEnvelope_GateAllows(t *testing.T) {
	t.Parallel()
	stub := testkit.NewStubTool("read", "")
	stub.Result = "content"
	executor := testkit.NewStubExecutor(stub)
	gate := testkit.NewStubSecurityGate(true, "")

	env, err := middleware.NewBuilder(executor).WithGate(gate).Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := env.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "content" {
		t.Errorf("result = %q, want content", result)
	}
}

func TestEnvelope_EnricherAppends(t *testing.T) {
	t.Parallel()
	stub := testkit.NewStubTool("read", "")
	stub.Result = "file data"
	executor := testkit.NewStubExecutor(stub)
	enricher := &testkit.StubEnricher{Result: "symbols: 42"}

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithEnricher(enricher).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := env.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "file data\n\nsymbols: 42" {
		t.Errorf("result = %q, want enriched output", result)
	}
	if enricher.Calls != 1 {
		t.Errorf("enricher.Calls = %d, want 1", enricher.Calls)
	}
}

func TestEnvelope_RecorderRecords(t *testing.T) {
	t.Parallel()
	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))
	recorder := &testkit.StubRecorder{}

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithRecorder(recorder).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	env.Execute(context.Background(), "read", json.RawMessage(`{}`)) //nolint:errcheck

	if len(recorder.Records) != 1 {
		t.Fatalf("recorder.Records = %d, want 1", len(recorder.Records))
	}
	if recorder.Records[0].Tool != "read" {
		t.Errorf("recorded tool = %q, want read", recorder.Records[0].Tool)
	}
}

func TestBuilder_RequiresSecurityGate(t *testing.T) {
	t.Parallel()
	executor := testkit.NewStubExecutor()

	// No gate → Build fails.
	_, err := middleware.NewBuilder(executor).Build()
	if !errors.Is(err, middleware.ErrNoSecurityGate) {
		t.Errorf("expected ErrNoSecurityGate, got %v", err)
	}

	// Non-security gate → Build still fails.
	regularGate := &testkit.StubGate{Allow: true}
	_, err = middleware.NewBuilder(executor).WithGate(regularGate).Build()
	if !errors.Is(err, middleware.ErrNoSecurityGate) {
		t.Errorf("expected ErrNoSecurityGate with regular gate, got %v", err)
	}

	// SecurityGate → Build succeeds.
	_, err = middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		Build()
	if err != nil {
		t.Errorf("expected success with SecurityGate, got %v", err)
	}
}

func TestEnvelope_ImplementsExecutor(t *testing.T) {
	t.Parallel()
	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", "Read"))

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Envelope must satisfy tool.Executor.
	var _ tool.Executor = env

	// All/Names delegate to wrapped executor.
	if len(env.All()) != 1 {
		t.Errorf("All() = %d, want 1", len(env.All()))
	}
	names := env.Names()
	if len(names) != 1 || names[0] != "read" {
		t.Errorf("Names() = %v, want [read]", names)
	}
}

func TestPolicyGate_AllowsAndDenies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	// Enforcer allows.
	enforcer := &testkit.StubEnforcer{}
	gate := middleware.NewPolicyGate(enforcer, testkit.AllowAllToken())

	v, err := gate.Check(ctx, "read", nil)
	if err != nil || !v.Allowed {
		t.Errorf("expected allowed, got %v %v", v, err)
	}

	// Enforcer denies.
	enforcer.Err = errors.New("path not writable")
	v, err = gate.Check(ctx, "write", nil)
	if err != nil {
		t.Fatal(err)
	}
	if v.Allowed {
		t.Error("expected denied")
	}
	if v.Reason != "path not writable" {
		t.Errorf("reason = %q", v.Reason)
	}
}
