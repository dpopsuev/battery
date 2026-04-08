// Package event defines the canonical event type and EventLog interface
// for append-only event sourcing across the Aeon ecosystem.
//
// Consumers (Djinn, Troupe, Origami) depend on this interface.
// Implementations (Ring, DurableBus) are adapters.
package event

import "time"

// Event captures a single action, decision, or state change.
// Immutable once emitted — append-only semantics.
type Event struct {
	ID        string            `json:"id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Timestamp time.Time         `json:"ts"`
	Source    string            `json:"source"`
	Kind      string            `json:"kind"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// EventLog is an append-only event log with sequential indexing.
// Implementations must be safe for concurrent use.
type EventLog interface {
	// Emit appends an event and returns its sequential index (0-based).
	// Timestamp is set automatically if zero.
	Emit(e Event) int

	// Since returns all events from index onward (inclusive).
	// Returns nil if index >= Len(). Negative index returns all.
	// The returned slice is a copy — callers may mutate it.
	Since(index int) []Event

	// Len returns the total number of events emitted.
	Len() int

	// OnEmit registers a callback invoked on every Emit.
	// Multiple callbacks are supported. Callbacks must be fast.
	OnEmit(fn func(Event))
}
