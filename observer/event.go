// Package observer defines tracing, event recording, and archive/diff for agent observability.
package observer

import "time"

// Component identifies the source of a trace event.
type Component string

const (
	ComponentMCP    Component = "mcp"
	ComponentAgent  Component = "agent"
	ComponentSignal Component = "signal"
	ComponentTool   Component = "tool"
	ComponentTUI    Component = "tui"
	ComponentReview Component = "review"
)

// ActionDoneSuffix is appended to action names when a round-trip completes.
const ActionDoneSuffix = "_done"

// Event is a single trace event for cross-component observability.
type Event struct {
	ID        string            `json:"id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Timestamp time.Time         `json:"ts"`
	Component Component         `json:"component"`
	Action    string            `json:"action"`
	Server    string            `json:"server,omitempty"`
	Tool      string            `json:"tool,omitempty"`
	Detail    string            `json:"detail"`
	Latency   time.Duration     `json:"latency,omitzero"`
	Error     bool              `json:"error,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// RingStats summarizes a Ring buffer's state.
type RingStats struct {
	Capacity int       `json:"capacity"`
	Count    int       `json:"count"`
	Oldest   time.Time `json:"oldest,omitzero"`
	Newest   time.Time `json:"newest,omitzero"`
}
