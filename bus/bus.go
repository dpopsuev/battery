// Package bus defines event primitives for the EDA agent architecture.
// Three buses: Motor (commands OUT), Sense (observations IN), Signal (telemetry).
package bus

import (
	"encoding/json"
	"time"
)

// Event is the base type for all bus events.
type Event struct {
	Type          string        `json:"type"`
	CorrelationID string        `json:"correlationId"`
	Timestamp     time.Time     `json:"timestamp"`
	Elapsed       time.Duration `json:"elapsed"`
}

// MotorEvent is a command flowing OUT from the Reasoner to organs.
type MotorEvent struct {
	Event
	Payload json.RawMessage `json:"payload"`
}

// SenseEvent is an observation flowing IN from organs to the Reasoner.
type SenseEvent struct {
	Event
	Payload      json.RawMessage `json:"payload"`
	IsError      bool            `json:"isError,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	IsFinal      bool            `json:"isFinal"`
}

// SignalEvent is Reasoner telemetry broadcast to observers.
type SignalEvent struct {
	Event
	Payload json.RawMessage `json:"payload"`
}

// Publisher sends events to a bus.
type Publisher[E any] interface {
	Publish(event E)
}

// Subscriber receives events from a bus.
type Subscriber[E any] interface {
	Subscribe(eventType string, handler func(E)) (unsubscribe func())
}

// Bus combines publishing and subscribing.
type Bus[E any] interface {
	Publisher[E]
	Subscriber[E]
}
