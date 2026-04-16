package middleware_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/battery/middleware"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

func TestCircuitBreaker_AllowsWhenClosed(t *testing.T) {
	cb := middleware.NewCircuitBreaker(3, time.Second, time.Second)
	if err := cb.Allow("read"); err != nil {
		t.Errorf("expected allow, got %v", err)
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cb := middleware.NewCircuitBreaker(2, time.Second, 100*time.Millisecond)

	cb.RecordFailure("read")
	cb.RecordFailure("read")

	err := cb.Allow("read")
	if !errors.Is(err, middleware.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterCooldown(t *testing.T) {
	cb := middleware.NewCircuitBreaker(1, time.Second, 10*time.Millisecond)

	cb.RecordFailure("read")
	if err := cb.Allow("read"); !errors.Is(err, middleware.ErrCircuitOpen) {
		t.Fatal("expected open")
	}

	time.Sleep(15 * time.Millisecond)

	// Half-open — one probe allowed.
	if err := cb.Allow("read"); err != nil {
		t.Errorf("expected half-open allow, got %v", err)
	}
}

func TestCircuitBreaker_ClosesOnSuccess(t *testing.T) {
	cb := middleware.NewCircuitBreaker(1, time.Second, 10*time.Millisecond)

	cb.RecordFailure("read")
	time.Sleep(15 * time.Millisecond)
	_ = cb.Allow("read") // half-open
	cb.RecordSuccess("read")

	// Should be closed now.
	if err := cb.Allow("read"); err != nil {
		t.Errorf("expected closed after success, got %v", err)
	}
}

func TestCircuitBreaker_IsolatesTools(t *testing.T) {
	cb := middleware.NewCircuitBreaker(1, time.Second, time.Second)

	cb.RecordFailure("read")

	if err := cb.Allow("read"); !errors.Is(err, middleware.ErrCircuitOpen) {
		t.Error("read should be open")
	}
	if err := cb.Allow("write"); err != nil {
		t.Errorf("write should be unaffected, got %v", err)
	}
}

func TestRateLimiter_AllowsWithinLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(3, time.Second)

	for range 3 {
		if err := rl.Allow("read"); err != nil {
			t.Fatalf("expected allow, got %v", err)
		}
	}
}

func TestRateLimiter_RejectsOverLimit(t *testing.T) {
	rl := middleware.NewRateLimiter(2, time.Second)

	_ = rl.Allow("read")
	_ = rl.Allow("read")

	err := rl.Allow("read")
	if !errors.Is(err, middleware.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestRateLimiter_ResetsAfterWindow(t *testing.T) {
	rl := middleware.NewRateLimiter(1, 10*time.Millisecond)

	_ = rl.Allow("read")
	if err := rl.Allow("read"); !errors.Is(err, middleware.ErrRateLimited) {
		t.Fatal("expected rate limited")
	}

	time.Sleep(15 * time.Millisecond)

	if err := rl.Allow("read"); err != nil {
		t.Errorf("expected allow after window reset, got %v", err)
	}
}

func TestRateLimiter_IsolatesTools(t *testing.T) {
	rl := middleware.NewRateLimiter(1, time.Second)

	_ = rl.Allow("read")
	if err := rl.Allow("write"); err != nil {
		t.Errorf("write should be unaffected, got %v", err)
	}
}

func TestEnvelope_CircuitBreaker_Integration(t *testing.T) {
	t.Parallel()
	failTool := testkit.NewStubTool("flaky", "")
	failTool.Err = errors.New("transient failure")

	executor := testkit.NewStubExecutor(failTool)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithCircuitBreaker(2, time.Second, 100*time.Millisecond).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	// Two failures.
	_, _ = env.Execute(context.Background(), "flaky", nil)
	_, _ = env.Execute(context.Background(), "flaky", nil)

	// Third call should be circuit open.
	_, err = env.Execute(context.Background(), "flaky", nil)
	if !errors.Is(err, middleware.ErrCircuitOpen) {
		t.Errorf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestEnvelope_RateLimit_Integration(t *testing.T) {
	t.Parallel()
	stub := testkit.NewStubTool("read", "")
	executor := testkit.NewStubExecutor(stub)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithRateLimit(2, time.Second).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_, _ = env.Execute(context.Background(), "read", nil)
	_, _ = env.Execute(context.Background(), "read", nil)

	_, err = env.Execute(context.Background(), "read", nil)
	if !errors.Is(err, middleware.ErrRateLimited) {
		t.Errorf("expected ErrRateLimited, got %v", err)
	}
}

func TestEnvelope_Retry_Integration(t *testing.T) {
	t.Parallel()

	callCount := 0
	failTool := testkit.NewStubTool("flaky", "")
	failTool.Err = errors.New("transient")

	executor := testkit.NewStubExecutor(failTool)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithRetry(3, middleware.FixedBackoff(1*time.Millisecond), func(_ error) bool { return true }).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.Execute(context.Background(), "flaky", nil)
	if err == nil {
		t.Error("expected error after retries exhausted")
	}

	// StubTool was called 3 times (max attempts).
	callCount = failTool.Calls
	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}

	_ = tool.TextResult("") // keep tool import
}

func TestEnvelope_Retry_SkipsPermanentError(t *testing.T) {
	t.Parallel()

	failTool := testkit.NewStubTool("perm", "")
	failTool.Err = errors.New("permanent failure")

	executor := testkit.NewStubExecutor(failTool)

	env, err := middleware.NewBuilder(executor).
		WithGate(testkit.NewStubSecurityGate(true, "")).
		WithRetry(3, middleware.FixedBackoff(1*time.Millisecond), func(err error) bool {
			return err.Error() != "permanent failure"
		}).
		Build()
	if err != nil {
		t.Fatal(err)
	}

	_, err = env.Execute(context.Background(), "perm", nil)
	if err == nil {
		t.Error("expected error")
	}

	// Should have been called only once — permanent error, no retry.
	if failTool.Calls != 1 {
		t.Errorf("expected 1 call (no retry for permanent), got %d", failTool.Calls)
	}
}
