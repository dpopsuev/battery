package observer_test

import (
	"testing"
	"time"

	"github.com/dpopsuev/battery/observer"
)

func TestAnalyzeRing_ErrorRates(t *testing.T) {
	ring := observer.NewRing(100)

	// Tool "read": 3 calls, 1 error = 33% error rate.
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "read"})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "read"})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "read"})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call_done", Tool: "read", Latency: 10 * time.Millisecond})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call_done", Tool: "read", Latency: 20 * time.Millisecond})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call_done", Tool: "read", Latency: 15 * time.Millisecond, Error: true})

	// Tool "write": 2 calls, 0 errors.
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "write"})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "write"})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call_done", Tool: "write", Latency: 5 * time.Millisecond})
	ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call_done", Tool: "write", Latency: 8 * time.Millisecond})

	m := observer.AnalyzeRing(ring)

	if m.ToolCallCounts["read"] != 3 {
		t.Errorf("read calls = %d, want 3", m.ToolCallCounts["read"])
	}
	if m.ToolCallCounts["write"] != 2 {
		t.Errorf("write calls = %d, want 2", m.ToolCallCounts["write"])
	}

	// Error rate: read = 1/3 ≈ 0.333
	readErr := m.ToolErrorRates["read"]
	if readErr < 0.3 || readErr > 0.4 {
		t.Errorf("read error rate = %f, want ~0.333", readErr)
	}
	if m.ToolErrorRates["write"] != 0 {
		t.Errorf("write error rate = %f, want 0", m.ToolErrorRates["write"])
	}
}

func TestAnalyzeRing_LatencyPercentiles(t *testing.T) {
	ring := observer.NewRing(100)

	// 10 calls with increasing latency: 10ms, 20ms, ..., 100ms.
	for i := range 10 {
		ring.Append(observer.Event{Component: observer.ComponentTool, Action: "call", Tool: "analyze"})
		ring.Append(observer.Event{
			Component: observer.ComponentTool,
			Action:    "call_done",
			Tool:      "analyze",
			Latency:   time.Duration(i+1) * 10 * time.Millisecond,
		})
	}

	m := observer.AnalyzeRing(ring)

	p50 := m.ToolLatencyP50["analyze"]
	if p50 < 40*time.Millisecond || p50 > 60*time.Millisecond {
		t.Errorf("p50 = %v, want ~50ms", p50)
	}

	p95 := m.ToolLatencyP95["analyze"]
	if p95 < 90*time.Millisecond || p95 > 100*time.Millisecond {
		t.Errorf("p95 = %v, want ~95-100ms", p95)
	}
}

func TestAnalyzeRing_CacheHitRatio(t *testing.T) {
	ring := observer.NewRing(100)

	ring.Append(observer.Event{Action: "cache_hit", Tool: "read"})
	ring.Append(observer.Event{Action: "cache_hit", Tool: "read"})
	ring.Append(observer.Event{Action: "cache_hit", Tool: "read"})
	ring.Append(observer.Event{Action: "cache_miss", Tool: "read"})

	m := observer.AnalyzeRing(ring)

	if m.CacheHitRatio != 0.75 {
		t.Errorf("cache hit ratio = %f, want 0.75", m.CacheHitRatio)
	}
}

func TestAnalyzeRing_CacheHitRatio_NoCacheEvents(t *testing.T) {
	ring := observer.NewRing(100)

	ring.Append(observer.Event{Action: "call", Tool: "read"})

	m := observer.AnalyzeRing(ring)

	if m.CacheHitRatio != -1 {
		t.Errorf("cache hit ratio = %f, want -1 (no cache events)", m.CacheHitRatio)
	}
}

func TestAnalyzeRing_EmptyRing(t *testing.T) {
	ring := observer.NewRing(100)
	m := observer.AnalyzeRing(ring)

	if len(m.ToolCallCounts) != 0 {
		t.Errorf("expected empty call counts, got %v", m.ToolCallCounts)
	}
	if m.CacheHitRatio != -1 {
		t.Errorf("cache hit ratio = %f, want -1", m.CacheHitRatio)
	}
}

func TestAnalyzeRing_GaugeTotals(t *testing.T) {
	ring := observer.NewRing(100)

	ring.Append(observer.Event{
		Action: "gauge", Tool: "analyze",
		Metadata: map[string]string{"tokens_in": "42 tokens", "bytes": "1024 bytes"},
	})
	ring.Append(observer.Event{
		Action: "gauge", Tool: "analyze",
		Metadata: map[string]string{"tokens_in": "8 tokens"},
	})

	m := observer.AnalyzeRing(ring)

	if m.GaugeTotals["analyze"]["tokens_in"] != 50 {
		t.Errorf("tokens_in total = %f, want 50", m.GaugeTotals["analyze"]["tokens_in"])
	}
	if m.GaugeTotals["analyze"]["bytes"] != 1024 {
		t.Errorf("bytes total = %f, want 1024", m.GaugeTotals["analyze"]["bytes"])
	}
}

func TestAnalyzeEvents_Direct(t *testing.T) {
	events := []observer.Event{
		{Action: "call", Tool: "lint"},
		{Action: "call_done", Tool: "lint", Latency: 50 * time.Millisecond},
	}

	m := observer.AnalyzeEvents(events)

	if m.ToolCallCounts["lint"] != 1 {
		t.Errorf("lint calls = %d, want 1", m.ToolCallCounts["lint"])
	}
	if m.ToolLatencyP50["lint"] != 50*time.Millisecond {
		t.Errorf("lint p50 = %v, want 50ms", m.ToolLatencyP50["lint"])
	}
}
