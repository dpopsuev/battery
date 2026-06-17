package nerve

import (
	"sync"
	"time"

	"github.com/dpopsuev/battery/bus"
)

// Option configures an InProcessNerve.
type Option func(*InProcessNerve)

// WithWatchdog enables a stall watchdog that calls onStall when no Pulse
// is received for stallDuration.
func WithWatchdog(stallDuration time.Duration, onStall func()) Option {
	return func(n *InProcessNerve) {
		n.stallDuration = stallDuration
		n.onStall = onStall
	}
}

// InProcessNerve composes three InProcessBus instances with an optional watchdog.
type InProcessNerve struct {
	motor   *bus.InProcessBus[bus.MotorEvent]
	sense   *bus.InProcessBus[bus.SenseEvent]
	signal  *bus.InProcessBus[bus.SignalEvent]
	tracker *bus.CorrelationTracker

	stallDuration time.Duration
	onStall       func()
	watchdogMu    sync.Mutex
	watchdogTimer *time.Timer
	disposed      bool
}

// New creates an InProcessNerve.
func New(opts ...Option) *InProcessNerve {
	n := &InProcessNerve{
		motor:   bus.NewInProcessBus[bus.MotorEvent](),
		sense:   bus.NewInProcessBus[bus.SenseEvent](),
		signal:  bus.NewInProcessBus[bus.SignalEvent](),
		tracker: bus.NewCorrelationTracker(),
	}
	for _, o := range opts {
		o(n)
	}
	if n.stallDuration > 0 && n.onStall != nil {
		n.watchdogTimer = time.AfterFunc(n.stallDuration, n.onStall)
	}
	return n
}

// Motor returns the motor bus.
func (n *InProcessNerve) Motor() bus.Bus[bus.MotorEvent] { return n.motor }

// Sense returns the sense bus.
func (n *InProcessNerve) Sense() bus.Bus[bus.SenseEvent] { return n.sense }

// Signal returns the signal bus.
func (n *InProcessNerve) Signal() bus.Bus[bus.SignalEvent] { return n.signal }

// Pulse resets the stall watchdog.
func (n *InProcessNerve) Pulse() {
	n.watchdogMu.Lock()
	defer n.watchdogMu.Unlock()
	if n.watchdogTimer != nil && !n.disposed {
		n.watchdogTimer.Reset(n.stallDuration)
	}
}

// Dispose stops the watchdog. Safe to call multiple times.
func (n *InProcessNerve) Dispose() {
	n.watchdogMu.Lock()
	defer n.watchdogMu.Unlock()
	n.disposed = true
	if n.watchdogTimer != nil {
		n.watchdogTimer.Stop()
	}
}

// AsNerve returns the Nerve interface view.
func (n *InProcessNerve) AsNerve() Nerve { return n }
