package observer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// ToolHook receives core tool lifecycle events.
type ToolHook interface {
	OnToolCall(ctx context.Context, toolName string, input json.RawMessage)
	OnToolResult(ctx context.Context, toolName string, result tool.Result, err error, elapsed time.Duration)
}

// GaugeHook receives tool measurement events.
type GaugeHook interface {
	OnGaugeMeasurement(ctx context.Context, toolName string, measurements []tool.Measurement)
}

// CacheHook receives cache hit/miss events.
type CacheHook interface {
	OnCacheHit(ctx context.Context, toolName, key string)
	OnCacheMiss(ctx context.Context, toolName, key string)
}

// RingHook adapts a Ring into lifecycle hooks, translating calls into Event appends.
// Implements ToolHook, GaugeHook, and CacheHook.
type RingHook struct {
	ring *Ring
}

// NewRingHook returns a hook that writes tool events to the given Ring.
func NewRingHook(r *Ring) *RingHook {
	return &RingHook{ring: r}
}

var _ ToolHook = (*RingHook)(nil)
var _ GaugeHook = (*RingHook)(nil)
var _ CacheHook = (*RingHook)(nil)

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

// MultiHook fans out hook calls to multiple hooks.
// Each target is checked via type assertion — targets only receive
// events for the interfaces they implement.
type MultiHook struct {
	targets []any
}

// NewMultiHook returns a hook that forwards to all given targets.
// Each target should implement one or more of ToolHook, GaugeHook, CacheHook.
func NewMultiHook(targets ...any) *MultiHook {
	return &MultiHook{targets: targets}
}

var _ ToolHook = (*MultiHook)(nil)
var _ GaugeHook = (*MultiHook)(nil)
var _ CacheHook = (*MultiHook)(nil)

func (m *MultiHook) OnToolCall(ctx context.Context, toolName string, input json.RawMessage) {
	for _, t := range m.targets {
		if h, ok := t.(ToolHook); ok {
			h.OnToolCall(ctx, toolName, input)
		}
	}
}

func (m *MultiHook) OnToolResult(ctx context.Context, toolName string, result tool.Result, err error, elapsed time.Duration) {
	for _, t := range m.targets {
		if h, ok := t.(ToolHook); ok {
			h.OnToolResult(ctx, toolName, result, err, elapsed)
		}
	}
}

func (m *MultiHook) OnGaugeMeasurement(ctx context.Context, toolName string, measurements []tool.Measurement) {
	for _, t := range m.targets {
		if h, ok := t.(GaugeHook); ok {
			h.OnGaugeMeasurement(ctx, toolName, measurements)
		}
	}
}

func (m *MultiHook) OnCacheHit(ctx context.Context, toolName, key string) {
	for _, t := range m.targets {
		if h, ok := t.(CacheHook); ok {
			h.OnCacheHit(ctx, toolName, key)
		}
	}
}

func (m *MultiHook) OnCacheMiss(ctx context.Context, toolName, key string) {
	for _, t := range m.targets {
		if h, ok := t.(CacheHook); ok {
			h.OnCacheMiss(ctx, toolName, key)
		}
	}
}
