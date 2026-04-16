package middleware

import (
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

// Sentinel errors for resilience middleware.
var (
	ErrCircuitOpen = errors.New("battery: circuit breaker open — tool temporarily unavailable")
	ErrRateLimited = errors.New("battery: rate limit exceeded")
)

// CircuitBreaker tracks per-tool failure state.
// Three states: closed (normal), open (rejecting), half-open (probing).
type CircuitBreaker struct {
	mu          sync.Mutex
	maxFailures int
	window      time.Duration
	cooldown    time.Duration
	tools       map[string]*circuitState
}

type circuitState struct {
	failures  int
	lastFail  time.Time
	openUntil time.Time
}

// NewCircuitBreaker creates a circuit breaker.
// maxFailures within window → open. After cooldown → half-open (one probe).
func NewCircuitBreaker(maxFailures int, window, cooldown time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		maxFailures: maxFailures,
		window:      window,
		cooldown:    cooldown,
		tools:       make(map[string]*circuitState),
	}
}

// Allow checks if a tool call is permitted.
func (cb *CircuitBreaker) Allow(toolName string) error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cs, ok := cb.tools[toolName]
	if !ok {
		return nil // no state = closed
	}

	now := time.Now()

	// Circuit is open — check if cooldown expired.
	if !cs.openUntil.IsZero() && now.Before(cs.openUntil) {
		return ErrCircuitOpen
	}

	// Cooldown expired — allow one probe (half-open).
	if !cs.openUntil.IsZero() {
		cs.openUntil = time.Time{} // reset, will re-open on failure
		cs.failures = 0
	}

	return nil
}

// RecordSuccess resets the failure count for a tool.
func (cb *CircuitBreaker) RecordSuccess(toolName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.tools, toolName)
}

// RecordFailure increments the failure count. Opens circuit if threshold reached.
func (cb *CircuitBreaker) RecordFailure(toolName string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	now := time.Now()
	cs, ok := cb.tools[toolName]
	if !ok {
		cs = &circuitState{}
		cb.tools[toolName] = cs
	}

	// Reset if outside window.
	if now.Sub(cs.lastFail) > cb.window {
		cs.failures = 0
	}

	cs.failures++
	cs.lastFail = now

	if cs.failures >= cb.maxFailures {
		cs.openUntil = now.Add(cb.cooldown)
	}
}

// RetryPolicy defines how to retry failed tool calls.
type RetryPolicy struct {
	MaxAttempts int
	Backoff     BackoffFunc
	IsTransient func(error) bool // returns true if error is retryable
}

// BackoffFunc returns the delay before the nth retry (0-indexed).
type BackoffFunc func(attempt int) time.Duration

// ExponentialBackoff returns a BackoffFunc with exponential delay + jitter.
func ExponentialBackoff(base time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		delay := base * (1 << attempt)
		jitter := time.Duration(rand.Int64N(int64(delay) / 4)) //nolint:gosec // jitter doesn't need crypto
		return delay + jitter
	}
}

// FixedBackoff returns a BackoffFunc with constant delay.
func FixedBackoff(delay time.Duration) BackoffFunc {
	return func(_ int) time.Duration { return delay }
}

// RateLimiter tracks call counts per tool within a time window.
type RateLimiter struct {
	mu       sync.Mutex
	maxCalls int
	window   time.Duration
	tools    map[string]*rateBucket
}

type rateBucket struct {
	count       int
	windowStart time.Time
}

// NewRateLimiter creates a rate limiter.
func NewRateLimiter(maxCalls int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		maxCalls: maxCalls,
		window:   window,
		tools:    make(map[string]*rateBucket),
	}
}

// Allow checks if a tool call is within the rate limit. Increments counter.
func (rl *RateLimiter) Allow(toolName string) error {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	rb, ok := rl.tools[toolName]
	if !ok {
		rl.tools[toolName] = &rateBucket{count: 1, windowStart: now}
		return nil
	}

	// Window expired — reset.
	if now.Sub(rb.windowStart) > rl.window {
		rb.count = 1
		rb.windowStart = now
		return nil
	}

	if rb.count >= rl.maxCalls {
		return ErrRateLimited
	}

	rb.count++
	return nil
}
