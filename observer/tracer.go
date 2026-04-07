package observer

import "time"

// Tracer is a component-scoped facade for recording trace events.
type Tracer struct {
	ring      *Ring
	component Component
}

// For creates a Tracer scoped to a component.
func (r *Ring) For(component Component) *Tracer {
	if r == nil {
		return nil
	}
	return &Tracer{ring: r, component: component}
}

// Begin starts a timed round-trip. Call End() when done.
func (t *Tracer) Begin(action, detail string) *RoundTrip {
	if t == nil {
		return &RoundTrip{nop: true}
	}
	id := t.ring.Append(Event{
		Component: t.component,
		Action:    action,
		Detail:    detail,
	})
	return &RoundTrip{
		tracer: t,
		id:     id,
		start:  time.Now(),
		action: action,
		detail: detail,
	}
}

// Event records a point-in-time event (no duration).
func (t *Tracer) Event(action, detail string) {
	if t == nil {
		return
	}
	t.ring.Append(Event{
		Component: t.component,
		Action:    action,
		Detail:    detail,
	})
}

// RoundTrip tracks a timed operation with parent-child correlation.
type RoundTrip struct {
	tracer   *Tracer
	id       string
	parentID string
	start    time.Time
	action   string
	detail   string
	server   string
	tool     string
	nop      bool
}

// WithServer sets the server field for the completion event.
func (s *RoundTrip) WithServer(server string) *RoundTrip {
	s.server = server
	return s
}

// WithTool sets the tool field for the completion event.
func (s *RoundTrip) WithTool(tool string) *RoundTrip {
	s.tool = tool
	return s
}

// End completes the round-trip, recording duration.
func (s *RoundTrip) End() {
	if s.nop {
		return
	}
	s.tracer.ring.Append(Event{
		ParentID:  s.id,
		Component: s.tracer.component,
		Action:    s.action + ActionDoneSuffix,
		Detail:    s.detail,
		Server:    s.server,
		Tool:      s.tool,
		Latency:   time.Since(s.start),
	})
}

// EndWithError completes the round-trip, marking it as an error.
func (s *RoundTrip) EndWithError() {
	if s.nop {
		return
	}
	s.tracer.ring.Append(Event{
		ParentID:  s.id,
		Component: s.tracer.component,
		Action:    s.action + ActionDoneSuffix,
		Detail:    s.detail,
		Server:    s.server,
		Tool:      s.tool,
		Latency:   time.Since(s.start),
		Error:     true,
	})
}

// Child creates a correlated sub-event.
func (s *RoundTrip) Child(action, detail string) *RoundTrip {
	if s.nop {
		return &RoundTrip{nop: true}
	}
	id := s.tracer.ring.Append(Event{
		ParentID:  s.id,
		Component: s.tracer.component,
		Action:    action,
		Detail:    detail,
	})
	return &RoundTrip{
		tracer:   s.tracer,
		id:       id,
		parentID: s.id,
		start:    time.Now(),
		action:   action,
		detail:   detail,
	}
}

// ID returns the event ID of this round-trip.
func (s *RoundTrip) ID() string { return s.id }
