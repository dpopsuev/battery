package observer

import (
	"context"
	"encoding/json"
	"time"
)

// Hook receives notifications about tool lifecycle events.
// Implementations must be safe for concurrent use.
type Hook interface {
	OnToolCall(ctx context.Context, tool string, input json.RawMessage)
	OnToolResult(ctx context.Context, tool string, output string, err error, elapsed time.Duration)
}

// RingHook adapts a Ring into a Hook, translating hook calls into Event appends.
type RingHook struct {
	ring *Ring
}

// NewRingHook returns a Hook that writes tool events to the given Ring.
func NewRingHook(r *Ring) *RingHook {
	return &RingHook{ring: r}
}

// OnToolCall records a tool invocation event.
func (h *RingHook) OnToolCall(_ context.Context, tool string, _ json.RawMessage) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "call",
		Tool:      tool,
		Detail:    "tool invoked",
	})
}

// OnToolResult records a tool completion event with latency and error status.
func (h *RingHook) OnToolResult(_ context.Context, tool, _ string, err error, elapsed time.Duration) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "call" + ActionDoneSuffix,
		Tool:      tool,
		Detail:    "tool completed",
		Latency:   elapsed,
		Error:     err != nil,
	})
}

// MultiHook fans out hook calls to multiple Hooks.
type MultiHook struct {
	hooks []Hook
}

// NewMultiHook returns a Hook that forwards to all given hooks.
func NewMultiHook(hooks ...Hook) *MultiHook {
	return &MultiHook{hooks: hooks}
}

// OnToolCall forwards to all wrapped hooks.
func (m *MultiHook) OnToolCall(ctx context.Context, tool string, input json.RawMessage) {
	for _, h := range m.hooks {
		h.OnToolCall(ctx, tool, input)
	}
}

// OnToolResult forwards to all wrapped hooks.
func (m *MultiHook) OnToolResult(ctx context.Context, tool, output string, err error, elapsed time.Duration) {
	for _, h := range m.hooks {
		h.OnToolResult(ctx, tool, output, err, elapsed)
	}
}
