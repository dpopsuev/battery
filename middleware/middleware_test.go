package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dpopsuev/battery/cache"
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
	if result.Text() != "content" {
		t.Errorf("result = %q, want content", result.Text())
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
	if result.Text() != "file data\nsymbols: 42" {
		t.Errorf("result = %q, want enriched output", result.Text())
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

	env.Execute(context.Background(), "read", json.RawMessage(`{}`)) //nolint:errcheck // test — error not relevant to assertion

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

func TestDefaultRecorder_CalledWhenNoExplicit(t *testing.T) {
	rec := &testkit.StubRecorder{}
	middleware.SetDefaultRecorder(rec)
	defer middleware.SetDefaultRecorder(nil)

	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))
	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	env.Execute(context.Background(), "read", json.RawMessage(`{}`)) //nolint:errcheck // test — error not relevant to assertion

	if len(rec.Records) != 1 {
		t.Fatalf("default recorder calls = %d, want 1", len(rec.Records))
	}
	if rec.Records[0].Tool != "read" {
		t.Errorf("tool = %q, want read", rec.Records[0].Tool)
	}
}

func TestDefaultRecorder_NotCalledWhenExplicitSet(t *testing.T) {
	defRec := &testkit.StubRecorder{}
	middleware.SetDefaultRecorder(defRec)
	defer middleware.SetDefaultRecorder(nil)

	explicitRec := &testkit.StubRecorder{}
	executor := testkit.NewStubExecutor(testkit.NewStubTool("read", ""))
	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithRecorder(explicitRec).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	env.Execute(context.Background(), "read", json.RawMessage(`{}`)) //nolint:errcheck // test — error not relevant to assertion

	if len(defRec.Records) != 0 {
		t.Errorf("default recorder should NOT fire when explicit recorder is set, got %d calls", len(defRec.Records))
	}
	if len(explicitRec.Records) != 1 {
		t.Errorf("explicit recorder calls = %d, want 1", len(explicitRec.Records))
	}
}

func TestDefaultRecorder_NilByDefault(t *testing.T) {
	if middleware.DefaultRecorder() != nil {
		t.Error("default recorder should be nil before SetDefaultRecorder")
	}
}

func TestEnvelope_CacheHitSkipsExecute(t *testing.T) {
	t.Parallel()
	cacheable := &testkit.StubCacheableTool{
		StubTool: *testkit.NewStubTool("compute", ""),
		KeyFn: func(input json.RawMessage) (string, bool) {
			return "k:" + string(input), true
		},
	}
	cacheable.Result = "computed"

	executor := testkit.NewStubExecutor(cacheable)
	mc := cache.NewMemCache(cache.MemCacheConfig{MaxEntries: 10})
	hook := &testkit.StubHook{}

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithCache(mc, hook, 0).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// First call: miss.
	result, err := env.Execute(context.Background(), "compute", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("first call: text=%q, calls=%d", result.Text(), len(executor.Calls))
	t.Logf("cache events: %+v", hook.CacheEvents)

	// Verify miss event fired.
	if len(hook.CacheEvents) != 1 || hook.CacheEvents[0].Hit {
		t.Errorf("expected 1 miss event, got %+v", hook.CacheEvents)
	}

	// Second call: hit.
	result2, err := env.Execute(context.Background(), "compute", json.RawMessage(`{"x":1}`))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("second call: text=%q, calls=%d", result2.Text(), len(executor.Calls))
	t.Logf("cache events: %+v", hook.CacheEvents)

	if len(executor.Calls) != 1 {
		t.Errorf("expected 1 execute call (cache hit), got %d", len(executor.Calls))
	}
}

func TestEnvelope_MaxResultSize_Truncates(t *testing.T) {
	t.Parallel()
	stub := testkit.NewStubTool("read", "")
	stub.Result = "a]very long output that should be truncated" // 44 bytes
	executor := testkit.NewStubExecutor(stub)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithMaxResultSize(10).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := env.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	text := result.Text()
	if len(text) <= 10 {
		// Should have truncated content + marker.
		t.Errorf("expected truncated output longer than 10 due to marker, got %d", len(text))
	}
	if text[:10] != "a]very lon" {
		t.Errorf("truncated prefix = %q", text[:10])
	}
	if !strings.Contains(text, "[battery: output truncated") {
		t.Errorf("missing truncation marker in: %q", text)
	}
}

func TestEnvelope_MaxResultSize_NoTruncateUnderLimit(t *testing.T) {
	t.Parallel()
	stub := testkit.NewStubTool("read", "")
	stub.Result = "short"
	executor := testkit.NewStubExecutor(stub)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithMaxResultSize(100).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := env.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "short" {
		t.Errorf("result = %q, want short (no truncation)", result.Text())
	}
}

func TestEnvelope_MaxResultSize_PerToolOverride(t *testing.T) {
	t.Parallel()
	stub := &testkit.StubMetadataTool{
		StubTool: *testkit.NewStubTool("read", ""),
		Meta:     tool.Metadata{MaxResultSize: 5},
	}
	stub.Result = "0123456789" // 10 bytes, exceeds per-tool limit of 5
	executor := testkit.NewStubExecutor(stub)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithMaxResultSize(100). // Default is 100, but per-tool is 5.
		Build()
	if err != nil {
		t.Fatal(err)
	}

	result, err := env.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	text2 := result.Text()
	if text2[:5] != "01234" {
		t.Errorf("truncated at per-tool limit: prefix = %q", text2[:5])
	}
	if !strings.Contains(text2, "[battery: output truncated") {
		t.Errorf("missing truncation marker: %q", text2)
	}
}
