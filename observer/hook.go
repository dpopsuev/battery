package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// Hook receives notifications about tool lifecycle events.
// Implementations must be safe for concurrent use.
type Hook interface {
	OnToolCall(ctx context.Context, toolName string, input json.RawMessage)
	OnToolResult(ctx context.Context, toolName string, result tool.Result, err error, elapsed time.Duration)
	OnGaugeMeasurement(ctx context.Context, toolName string, measurements []tool.Measurement)
	OnCacheHit(ctx context.Context, toolName, key string)
	OnCacheMiss(ctx context.Context, toolName, key string)
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
func (h *RingHook) OnToolCall(_ context.Context, toolName string, _ json.RawMessage) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "call",
		Tool:      toolName,
		Detail:    "tool invoked",
	})
}

// OnToolResult records a tool completion event with latency and error status.
func (h *RingHook) OnToolResult(_ context.Context, toolName string, _ tool.Result, err error, elapsed time.Duration) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "call" + ActionDoneSuffix,
		Tool:      toolName,
		Detail:    "tool completed",
		Latency:   elapsed,
		Error:     err != nil,
	})
}

// OnGaugeMeasurement records tool measurements as a gauge event.
func (h *RingHook) OnGaugeMeasurement(_ context.Context, toolName string, measurements []tool.Measurement) {
	meta := make(map[string]string, len(measurements))
	for _, m := range measurements {
		meta[m.Name] = fmt.Sprintf("%g %s", m.Value, m.Unit)
	}
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "gauge",
		Tool:      toolName,
		Detail:    "tool measurement",
		Metadata:  meta,
	})
}

// OnCacheHit records a cache hit event.
func (h *RingHook) OnCacheHit(_ context.Context, toolName, key string) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "cache_hit",
		Tool:      toolName,
		Detail:    key,
	})
}

// OnCacheMiss records a cache miss event.
func (h *RingHook) OnCacheMiss(_ context.Context, toolName, key string) {
	h.ring.Append(Event{
		Component: ComponentTool,
		Action:    "cache_miss",
		Tool:      toolName,
		Detail:    key,
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
func (m *MultiHook) OnToolCall(ctx context.Context, toolName string, input json.RawMessage) {
	for _, h := range m.hooks {
		h.OnToolCall(ctx, toolName, input)
	}
}

// OnToolResult forwards to all wrapped hooks.
func (m *MultiHook) OnToolResult(ctx context.Context, toolName string, result tool.Result, err error, elapsed time.Duration) {
	for _, h := range m.hooks {
		h.OnToolResult(ctx, toolName, result, err, elapsed)
	}
}

// OnGaugeMeasurement forwards to all wrapped hooks.
func (m *MultiHook) OnGaugeMeasurement(ctx context.Context, toolName string, measurements []tool.Measurement) {
	for _, h := range m.hooks {
		h.OnGaugeMeasurement(ctx, toolName, measurements)
	}
}

// OnCacheHit forwards to all wrapped hooks.
func (m *MultiHook) OnCacheHit(ctx context.Context, toolName, key string) {
	for _, h := range m.hooks {
		h.OnCacheHit(ctx, toolName, key)
	}
}

// OnCacheMiss forwards to all wrapped hooks.
func (m *MultiHook) OnCacheMiss(ctx context.Context, toolName, key string) {
	for _, h := range m.hooks {
		h.OnCacheMiss(ctx, toolName, key)
	}
}
