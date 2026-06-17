package bus

import (
	"sync"
	"time"
)

// CorrelationTracker stamps events with elapsed time since first-seen correlation ID.
type CorrelationTracker struct {
	mu        sync.Mutex
	firstSeen map[string]time.Time
}

// NewCorrelationTracker creates a tracker.
func NewCorrelationTracker() *CorrelationTracker {
	return &CorrelationTracker{
		firstSeen: make(map[string]time.Time),
	}
}

// Stamp sets the event's Timestamp to now and Elapsed to the duration since
// the correlation ID was first seen (0 on first occurrence).
func (ct *CorrelationTracker) Stamp(event *Event) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	now := time.Now()
	event.Timestamp = now

	if first, ok := ct.firstSeen[event.CorrelationID]; ok {
		event.Elapsed = now.Sub(first)
	} else {
		ct.firstSeen[event.CorrelationID] = now
		event.Elapsed = 0
	}
}

// Evict removes a correlation ID from tracking.
func (ct *CorrelationTracker) Evict(correlationID string) {
	ct.mu.Lock()
	delete(ct.firstSeen, correlationID)
	ct.mu.Unlock()
}
