package observer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Ring is a thread-safe bounded circular buffer for trace events.
type Ring struct {
	mu     sync.RWMutex
	events []Event
	cap    int
	pos    int
	count  int
	nextID atomic.Int64
}

// NewRing creates a ring buffer with the given capacity.
func NewRing(capacity int) *Ring {
	return &Ring{
		events: make([]Event, capacity),
		cap:    capacity,
	}
}

// Append adds an event to the ring, assigning it an auto-generated ID.
// Returns the assigned ID.
func (r *Ring) Append(e Event) string {
	id := fmt.Sprintf("evt-%d", r.nextID.Add(1))
	e.ID = id
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}

	r.mu.Lock()
	r.events[r.pos] = e
	r.pos = (r.pos + 1) % r.cap
	if r.count < r.cap {
		r.count++
	}
	r.mu.Unlock()

	return id
}

// Last returns the most recent n events in chronological order.
func (r *Ring) Last(n int) []Event {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if n > r.count {
		n = r.count
	}
	out := make([]Event, n)
	start := (r.pos - n + r.cap) % r.cap
	for i := range n {
		out[i] = r.events[(start+i)%r.cap]
	}
	return out
}

// Since returns events with timestamps after t.
func (r *Ring) Since(t time.Time) []Event {
	all := r.Last(r.Stats().Count)
	var out []Event
	for _, e := range all {
		if e.Timestamp.After(t) {
			out = append(out, e)
		}
	}
	return out
}

// ByParent returns events with the given parent ID.
func (r *Ring) ByParent(parentID string) []Event {
	all := r.Last(r.Stats().Count)
	var out []Event
	for _, e := range all {
		if e.ParentID == parentID {
			out = append(out, e)
		}
	}
	return out
}

// ByComponent returns events matching the given component.
func (r *Ring) ByComponent(c Component) []Event {
	all := r.Last(r.Stats().Count)
	var out []Event
	for _, e := range all {
		if e.Component == c {
			out = append(out, e)
		}
	}
	return out
}

// Get returns a specific event by ID.
func (r *Ring) Get(id string) (Event, bool) {
	all := r.Last(r.Stats().Count)
	for _, e := range all {
		if e.ID == id {
			return e, true
		}
	}
	return Event{}, false
}

// Stats returns summary statistics of the ring buffer.
func (r *Ring) Stats() RingStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RingStats{Capacity: r.cap, Count: r.count}
	if r.count > 0 {
		oldest := (r.pos - r.count + r.cap) % r.cap
		stats.Oldest = r.events[oldest].Timestamp
		newest := (r.pos - 1 + r.cap) % r.cap
		stats.Newest = r.events[newest].Timestamp
	}
	return stats
}
