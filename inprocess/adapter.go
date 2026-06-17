// Package inprocess provides a zero-transport adapter for composing
// organs in the same process. Used for testing and single-process agents.
package inprocess

import (
	"context"
	"errors"

	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/organ"
)

// ErrAlreadyStarted is returned when Start is called twice.
var ErrAlreadyStarted = errors.New("inprocess: already started")

// Adapter composes multiple Organs around a shared InProcessNerve.
type Adapter struct {
	nerve    *nerve.InProcessNerve
	organs   []organ.Organ
	unmounts []func()
	started  bool
}

// New creates an adapter with the given nerve options.
func New(opts ...nerve.Option) *Adapter {
	return &Adapter{
		nerve: nerve.New(opts...),
	}
}

// Load adds organs to the adapter. Must be called before Start.
func (a *Adapter) Load(organs ...organ.Organ) {
	a.organs = append(a.organs, organs...)
}

// Start mounts all loaded organs on the shared nerve.
func (a *Adapter) Start(_ context.Context) error {
	if a.started {
		return ErrAlreadyStarted
	}
	a.started = true
	for _, o := range a.organs {
		unmount := o.Mount(a.nerve.AsNerve())
		a.unmounts = append(a.unmounts, unmount)
	}
	return nil
}

// Stop unmounts and closes all organs, then disposes the nerve.
func (a *Adapter) Stop(_ context.Context) error {
	for i := len(a.unmounts) - 1; i >= 0; i-- {
		a.unmounts[i]()
	}
	a.unmounts = nil
	for _, o := range a.organs {
		if err := o.Close(); err != nil {
			return err
		}
	}
	a.nerve.Dispose()
	a.started = false
	return nil
}

// Nerve returns the underlying nerve for direct access.
func (a *Adapter) Nerve() nerve.Nerve { return a.nerve.AsNerve() }

// PublishMotor injects a motor event into the shared nerve.
func (a *Adapter) PublishMotor(event bus.MotorEvent) {
	a.nerve.Motor().Publish(event)
}

// SubscribeSense subscribes to sense events on the shared nerve.
func (a *Adapter) SubscribeSense(eventType string, h func(bus.SenseEvent)) func() {
	return a.nerve.Sense().Subscribe(eventType, h)
}
