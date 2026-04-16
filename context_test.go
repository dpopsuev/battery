package battery_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dpopsuev/battery"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

func TestAgentInfo_RoundTrip(t *testing.T) {
	ctx := context.Background()
	info := battery.AgentInfo{Name: "djinn", Version: "v1.0", SessionID: "sess-42"}

	ctx = battery.ContextWithAgentInfo(ctx, info)
	got := battery.AgentInfoFromContext(ctx)

	if got == nil {
		t.Fatal("expected AgentInfo, got nil")
	}
	if got.Name != "djinn" || got.Version != "v1.0" || got.SessionID != "sess-42" {
		t.Errorf("AgentInfo = %+v", got)
	}
}

func TestAgentInfo_NilSafe(t *testing.T) {
	ctx := context.Background()
	got := battery.AgentInfoFromContext(ctx)
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestExecutor_RoundTrip(t *testing.T) {
	ctx := context.Background()
	stub := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))

	ctx = battery.ContextWithExecutor(ctx, stub)
	got := battery.ExecutorFromContext(ctx)

	if got == nil {
		t.Fatal("expected Executor, got nil")
	}
	names := got.Names()
	if len(names) != 1 || names[0] != "read" {
		t.Errorf("Names = %v", names)
	}
}

func TestExecutor_NilSafe(t *testing.T) {
	ctx := context.Background()
	got := battery.ExecutorFromContext(ctx)
	if got != nil {
		t.Error("expected nil")
	}
}

func TestTTL_Default(t *testing.T) {
	ctx := context.Background()
	ttl := battery.TTLFromContext(ctx)
	if ttl != 10 {
		t.Errorf("default TTL = %d, want 10", ttl)
	}
}

func TestTTL_Custom(t *testing.T) {
	ctx := battery.ContextWithTTL(context.Background(), 3)
	if battery.TTLFromContext(ctx) != 3 {
		t.Errorf("TTL = %d, want 3", battery.TTLFromContext(ctx))
	}
}

func TestTTL_Decrement(t *testing.T) {
	ctx := battery.ContextWithTTL(context.Background(), 3)

	var err error
	ctx, err = battery.DecrementTTL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if battery.TTLFromContext(ctx) != 2 {
		t.Errorf("TTL after decrement = %d, want 2", battery.TTLFromContext(ctx))
	}
}

func TestTTL_ExceedsAtZero(t *testing.T) {
	ctx := battery.ContextWithTTL(context.Background(), 1)

	ctx, err := battery.DecrementTTL(ctx)
	if err != nil {
		t.Fatal("TTL=1 should succeed on first decrement")
	}
	if battery.TTLFromContext(ctx) != 0 {
		t.Errorf("TTL = %d, want 0", battery.TTLFromContext(ctx))
	}

	_, err = battery.DecrementTTL(ctx)
	if !errors.Is(err, battery.ErrTTLExceeded) {
		t.Errorf("expected ErrTTLExceeded, got %v", err)
	}
}

func TestTTL_DefaultDecrement(t *testing.T) {
	ctx := context.Background()
	ctx, err := battery.DecrementTTL(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if battery.TTLFromContext(ctx) != 9 {
		t.Errorf("TTL = %d, want 9 (default 10 - 1)", battery.TTLFromContext(ctx))
	}
}

func TestExecutor_ToolCallsToolThroughContext(t *testing.T) {
	// Tool B — the inner tool.
	innerTool := testkit.NewStubTool("inner", "inner tool")
	innerTool.Result = "inner-result"

	// Tool A — reads executor from context, calls inner.
	outerTool := testkit.NewStubTool("outer", "outer tool")
	// We can't make StubTool call through context, but we can verify
	// the wiring: executor is in context, TTL decrements.

	executor := testkit.NewStubExecutor(outerTool, innerTool)
	ctx := battery.ContextWithExecutor(context.Background(), executor)
	ctx = battery.ContextWithTTL(ctx, 5)

	// Simulate outer tool reading executor from context.
	exec := battery.ExecutorFromContext(ctx)
	if exec == nil {
		t.Fatal("no executor in context")
	}

	// Decrement TTL before inner call.
	ctx, err := battery.DecrementTTL(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Call inner tool through context executor.
	result, err := exec.Execute(ctx, "inner", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "inner-result" {
		t.Errorf("inner result = %q", result.Text())
	}

	// TTL should be 4 now.
	if battery.TTLFromContext(ctx) != 4 {
		t.Errorf("TTL = %d, want 4", battery.TTLFromContext(ctx))
	}
}

func TestAgentInfo_CacheScopeIsolation(t *testing.T) {
	agent1 := battery.AgentInfo{Name: "agent-1", SessionID: "s1"}
	agent2 := battery.AgentInfo{Name: "agent-2", SessionID: "s2"}

	ctx1 := battery.ContextWithAgentInfo(context.Background(), agent1)
	ctx2 := battery.ContextWithAgentInfo(context.Background(), agent2)

	// Each context has its own identity.
	got1 := battery.AgentInfoFromContext(ctx1)
	got2 := battery.AgentInfoFromContext(ctx2)

	if got1.SessionID == got2.SessionID {
		t.Error("agent sessions should be isolated")
	}

	// A cache key scoped by agent would differ.
	key1 := got1.SessionID + ":tool:args"
	key2 := got2.SessionID + ":tool:args"
	if key1 == key2 {
		t.Error("cache keys should differ per agent")
	}

	_ = tool.TextResult("") // ensure tool import is used
}
