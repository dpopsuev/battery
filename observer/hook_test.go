package observer_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dpopsuev/battery/observer"
	"github.com/dpopsuev/battery/tool"
)

// HookContract validates any Hook implementation.
func HookContract(t *testing.T, newHook func() observer.Hook) {
	t.Helper()

	t.Run("OnToolCall", func(t *testing.T) {
		h := newHook()
		h.OnToolCall(context.Background(), "read", json.RawMessage(`{"path":"."}`))
	})

	t.Run("OnToolResult_Success", func(t *testing.T) {
		h := newHook()
		h.OnToolResult(context.Background(), "read", "file contents", nil, 50*time.Millisecond)
	})

	t.Run("OnToolResult_Error", func(t *testing.T) {
		h := newHook()
		h.OnToolResult(context.Background(), "read", "", errors.New("not found"), 10*time.Millisecond)
	})

	t.Run("OnGaugeMeasurement", func(t *testing.T) {
		h := newHook()
		h.OnGaugeMeasurement(context.Background(), "read", []tool.Measurement{
			{Name: "bytes", Value: 1024, Unit: "bytes"},
		})
	})
}

func TestRingHook(t *testing.T) {
	ring := observer.NewRing(100)

	HookContract(t, func() observer.Hook {
		return observer.NewRingHook(ring)
	})

	// Verify events were written to the ring.
	ring2 := observer.NewRing(100)
	h := observer.NewRingHook(ring2)

	h.OnToolCall(context.Background(), "analyze", json.RawMessage(`{}`))
	h.OnToolResult(context.Background(), "analyze", "done", nil, 100*time.Millisecond)

	events := ring2.Last(10)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}

	if events[0].Action != "call" {
		t.Errorf("first event action: got %q, want %q", events[0].Action, "call")
	}
	if events[0].Tool != "analyze" {
		t.Errorf("first event tool: got %q, want %q", events[0].Tool, "analyze")
	}

	if events[1].Action != "call_done" {
		t.Errorf("second event action: got %q, want %q", events[1].Action, "call_done")
	}
	if events[1].Latency != 100*time.Millisecond {
		t.Errorf("second event latency: got %v, want %v", events[1].Latency, 100*time.Millisecond)
	}
	if events[1].Error {
		t.Error("second event should not have error flag")
	}
}

func TestRingHook_Gauge(t *testing.T) {
	ring := observer.NewRing(100)
	h := observer.NewRingHook(ring)

	h.OnGaugeMeasurement(context.Background(), "analyze", []tool.Measurement{
		{Name: "tokens_in", Value: 42, Unit: "tokens"},
		{Name: "bytes_read", Value: 1024, Unit: "bytes"},
	})

	events := ring.Last(1)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	e := events[0]
	if e.Action != "gauge" {
		t.Errorf("action: got %q, want gauge", e.Action)
	}
	if e.Tool != "analyze" {
		t.Errorf("tool: got %q, want analyze", e.Tool)
	}
	if e.Metadata["tokens_in"] != "42 tokens" {
		t.Errorf("metadata[tokens_in] = %q", e.Metadata["tokens_in"])
	}
	if e.Metadata["bytes_read"] != "1024 bytes" {
		t.Errorf("metadata[bytes_read] = %q", e.Metadata["bytes_read"])
	}
}

func TestRingHook_Error(t *testing.T) {
	ring := observer.NewRing(100)
	h := observer.NewRingHook(ring)

	h.OnToolResult(context.Background(), "lint", "", errors.New("parse error"), 5*time.Millisecond)

	events := ring.Last(1)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if !events[0].Error {
		t.Error("expected error flag to be set")
	}
}

func TestMultiHook(t *testing.T) {
	ring1 := observer.NewRing(100)
	ring2 := observer.NewRing(100)
	multi := observer.NewMultiHook(observer.NewRingHook(ring1), observer.NewRingHook(ring2))

	multi.OnToolCall(context.Background(), "read", json.RawMessage(`{}`))
	multi.OnToolResult(context.Background(), "read", "ok", nil, 10*time.Millisecond)

	for i, ring := range []*observer.Ring{ring1, ring2} {
		events := ring.Last(10)
		if len(events) != 2 {
			t.Errorf("ring %d: expected 2 events, got %d", i, len(events))
		}
	}
}
