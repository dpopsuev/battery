package service

import "context"

// Controller receives Alerts and acts on them.
// The deterministic "efferent" half of the control loop.
// Examples: RelayManager, WorktreeManager, ReconcileController.
type Controller interface {
	Service
	Handle(ctx context.Context, alert Alert) error
	Subjects() []string // categories this controller handles
}
