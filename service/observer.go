package service

import "context"

// Alert is a finding from an Observer's check cycle.
type Alert struct {
	Category string         `json:"category"`
	Level    string         `json:"level"` // "info", "warning", "critical"
	Message  string         `json:"message"`
	Details  map[string]any `json:"details,omitempty"`
}

// Observer detects conditions and emits Alerts.
// The deterministic "afferent" half of the control loop.
// Examples: BudgetWatchdog, DeadlockWatchdog, DriftWatchdog.
type Observer interface {
	Service
	Check(ctx context.Context) []Alert
	Category() string
}
