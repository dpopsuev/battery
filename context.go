package battery

import (
	"context"
	"errors"

	"github.com/dpopsuev/battery/tool"
)

// ErrTTLExceeded is returned when the call depth budget is exhausted.
var ErrTTLExceeded = errors.New("battery: TTL exceeded — call depth budget exhausted")

// AgentInfo identifies the caller in a multi-agent context.
type AgentInfo struct {
	Name      string // from MCP Implementation.Name
	Version   string // from MCP Implementation.Version
	SessionID string // unique per connection
}

type agentInfoKey struct{}
type executorKey struct{}
type ttlKey struct{}

const defaultTTL = 10

// ContextWithAgentInfo attaches agent identity to the context.
func ContextWithAgentInfo(ctx context.Context, info AgentInfo) context.Context {
	return context.WithValue(ctx, agentInfoKey{}, &info)
}

// AgentInfoFromContext returns the agent identity, or nil if not set.
func AgentInfoFromContext(ctx context.Context) *AgentInfo {
	v := ctx.Value(agentInfoKey{})
	if v == nil {
		return nil
	}
	return v.(*AgentInfo)
}

// ContextWithExecutor attaches an Executor to the context.
// Tools call ExecutorFromContext to invoke other tools through the same pipeline.
func ContextWithExecutor(ctx context.Context, exec tool.Executor) context.Context {
	return context.WithValue(ctx, executorKey{}, exec)
}

// ExecutorFromContext returns the Executor from context, or nil if not set.
func ExecutorFromContext(ctx context.Context) tool.Executor {
	v := ctx.Value(executorKey{})
	if v == nil {
		return nil
	}
	return v.(tool.Executor)
}

// ContextWithTTL sets the call depth budget.
func ContextWithTTL(ctx context.Context, ttl int) context.Context {
	return context.WithValue(ctx, ttlKey{}, ttl)
}

// DecrementTTL decrements the TTL and returns the updated context.
// Returns ErrTTLExceeded when TTL reaches zero.
// If no TTL is set, uses defaultTTL (10).
func DecrementTTL(ctx context.Context) (context.Context, error) {
	ttl := ttlFromContext(ctx)
	if ttl <= 0 {
		return ctx, ErrTTLExceeded
	}
	return context.WithValue(ctx, ttlKey{}, ttl-1), nil
}

// TTLFromContext returns the current TTL, or defaultTTL if not set.
func TTLFromContext(ctx context.Context) int {
	return ttlFromContext(ctx)
}

func ttlFromContext(ctx context.Context) int {
	v := ctx.Value(ttlKey{})
	if v == nil {
		return defaultTTL
	}
	return v.(int)
}
