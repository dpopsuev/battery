package testkit

import (
	"sync"
	"testing"

	"github.com/dpopsuev/battery/event"
)

// RunEventLogContract verifies any EventLog implementation satisfies
// the behavioral contract. Every implementation must pass this.
func RunEventLogContract(t *testing.T, log event.EventLog) {
	t.Helper()

	t.Run("Emit_ReturnsSequentialIndices", func(t *testing.T) {
		i0 := log.Emit(event.Event{Source: "test", Kind: "a"})
		i1 := log.Emit(event.Event{Source: "test", Kind: "b"})
		i2 := log.Emit(event.Event{Source: "test", Kind: "c"})

		if i1 != i0+1 {
			t.Fatalf("indices not sequential: %d, %d", i0, i1)
		}
		if i2 != i1+1 {
			t.Fatalf("indices not sequential: %d, %d", i1, i2)
		}
	})

	t.Run("Since_ReturnsFromIndex", func(t *testing.T) {
		start := log.Len()
		log.Emit(event.Event{Source: "test", Kind: "x"})
		log.Emit(event.Event{Source: "test", Kind: "y"})

		events := log.Since(start)
		if len(events) != 2 {
			t.Fatalf("Since(%d) = %d events, want 2", start, len(events))
		}
		if events[0].Kind != "x" {
			t.Fatalf("events[0].Kind = %q, want x", events[0].Kind)
		}
		if events[1].Kind != "y" {
			t.Fatalf("events[1].Kind = %q, want y", events[1].Kind)
		}
	})

	t.Run("Since_BeyondLen_ReturnsNil", func(t *testing.T) {
		events := log.Since(log.Len())
		if events != nil {
			t.Fatalf("Since(Len()) = %d events, want nil", len(events))
		}
	})

	t.Run("Since_Negative_ReturnsAll", func(t *testing.T) {
		events := log.Since(-1)
		if len(events) != log.Len() {
			t.Fatalf("Since(-1) = %d, want %d", len(events), log.Len())
		}
	})

	t.Run("Since_ReturnsCopy", func(t *testing.T) {
		log.Emit(event.Event{Source: "test", Kind: "copy"})
		events := log.Since(0)
		events[0].Kind = "MUTATED"

		fresh := log.Since(0)
		if fresh[0].Kind == "MUTATED" {
			t.Fatal("Since must return a copy, not a reference to internal storage")
		}
	})

	t.Run("Len_MatchesEmitCount", func(t *testing.T) {
		before := log.Len()
		log.Emit(event.Event{Source: "test", Kind: "count"})
		after := log.Len()
		if after != before+1 {
			t.Fatalf("Len after Emit = %d, want %d", after, before+1)
		}
	})

	t.Run("OnEmit_CallbackFires", func(t *testing.T) {
		var received []event.Event
		var mu sync.Mutex
		log.OnEmit(func(e event.Event) {
			mu.Lock()
			received = append(received, e)
			mu.Unlock()
		})
		log.Emit(event.Event{Source: "test", Kind: "hook"})

		mu.Lock()
		defer mu.Unlock()
		found := false
		for _, e := range received {
			if e.Kind == "hook" {
				found = true
			}
		}
		if !found {
			t.Fatal("OnEmit callback did not fire")
		}
	})

	t.Run("Emit_SetsTimestamp", func(t *testing.T) {
		log.Emit(event.Event{Source: "test", Kind: "ts"})
		events := log.Since(log.Len() - 1)
		if events[0].Timestamp.IsZero() {
			t.Fatal("Emit should set Timestamp if zero")
		}
	})

	t.Run("ConcurrentEmit_Safe", func(t *testing.T) {
		var wg sync.WaitGroup
		before := log.Len()

		for range 20 {
			wg.Go(func() {
				log.Emit(event.Event{Source: "test", Kind: "concurrent"})
			})
		}
		wg.Wait()

		after := log.Len()
		if after != before+20 {
			t.Fatalf("Len = %d, want %d (20 concurrent emits)", after, before+20)
		}
	})
}
