// Package service defines lifecycle contracts for deterministic Actors.
//
// Three interfaces: Service (lifecycle), Observer (detect + emit),
// Controller (receive + act). All runtime processes in Djinn implement
// one of these. See DJN-NED-41 (Unified Actor Model).
package service

import "context"

// HealthStatus describes the current health of a service.
type HealthStatus string

const (
	Healthy  HealthStatus = "healthy"
	Degraded HealthStatus = "degraded"
	Failed   HealthStatus = "failed"
)

// HealthReport is the result of a health check.
type HealthReport struct {
	Status  HealthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

// Service is the lifecycle contract for all deterministic Actors.
// Agents (stochastic) and Operators (human) implement Actor.Perform()
// from Troupe; deterministic processes implement Service.
type Service interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Health() HealthReport
}
