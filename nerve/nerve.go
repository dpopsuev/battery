// Package nerve provides the unified bus view given to each organ on mount.
// A Nerve composes Motor, Sense, and Signal buses into a single handle.
package nerve

import "github.com/dpopsuev/battery/bus"

// Nerve is the unified view of all three buses.
type Nerve interface {
	Motor() bus.Bus[bus.MotorEvent]
	Sense() bus.Bus[bus.SenseEvent]
	Signal() bus.Bus[bus.SignalEvent]
	Pulse()
}

// Middleware wraps a Nerve to intercept events. Composable.
type Middleware func(Nerve) Nerve

// Chain composes middlewares left-to-right (outermost first).
func Chain(mws ...Middleware) Middleware {
	return func(n Nerve) Nerve {
		for i := len(mws) - 1; i >= 0; i-- {
			n = mws[i](n)
		}
		return n
	}
}
