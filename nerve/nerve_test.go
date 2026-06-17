package nerve_test

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/nerve"
)

func motor(typ string) bus.MotorEvent {
	return bus.MotorEvent{
		Event:   bus.Event{Type: typ, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	}
}

func sense(typ string) bus.SenseEvent {
	return bus.SenseEvent{
		Event:   bus.Event{Type: typ, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
		IsFinal: true,
	}
}

func TestInProcessNerve_MotorRouting(t *testing.T) {
	n := nerve.New()
	defer n.Dispose()

	var received string
	n.Motor().Subscribe("fs.read", func(e bus.MotorEvent) { received = e.Type })
	n.Motor().Publish(motor("fs.read"))

	if received != "fs.read" {
		t.Errorf("expected fs.read; got %q", received)
	}
}

func TestInProcessNerve_SenseRouting(t *testing.T) {
	n := nerve.New()
	defer n.Dispose()

	var received string
	n.Sense().Subscribe("fs.read", func(e bus.SenseEvent) { received = e.Type })
	n.Sense().Publish(sense("fs.read"))

	if received != "fs.read" {
		t.Errorf("expected fs.read; got %q", received)
	}
}

func TestInProcessNerve_SignalRouting(t *testing.T) {
	n := nerve.New()
	defer n.Dispose()

	var received string
	n.Signal().Subscribe("llm.chunk", func(e bus.SignalEvent) { received = e.Type })
	n.Signal().Publish(bus.SignalEvent{
		Event:   bus.Event{Type: "llm.chunk", CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{"text":"hello"}`),
	})

	if received != "llm.chunk" {
		t.Errorf("expected llm.chunk; got %q", received)
	}
}

func TestInProcessNerve_WatchdogFiresOnStall(t *testing.T) {
	var fired atomic.Bool
	n := nerve.New(nerve.WithWatchdog(20*time.Millisecond, func() { fired.Store(true) }))
	defer n.Dispose()

	time.Sleep(50 * time.Millisecond)
	if !fired.Load() {
		t.Error("watchdog should have fired after stall duration")
	}
}

func TestInProcessNerve_PulseResetsWatchdog(t *testing.T) {
	var fired atomic.Bool
	n := nerve.New(nerve.WithWatchdog(30*time.Millisecond, func() { fired.Store(true) }))
	defer n.Dispose()

	time.Sleep(15 * time.Millisecond)
	n.Pulse()
	time.Sleep(15 * time.Millisecond)
	n.Pulse()
	time.Sleep(15 * time.Millisecond)

	if fired.Load() {
		t.Error("watchdog should not fire when pulsed regularly")
	}
}

func TestInProcessNerve_DisposeStopsWatchdog(t *testing.T) {
	var fired atomic.Bool
	n := nerve.New(nerve.WithWatchdog(10*time.Millisecond, func() { fired.Store(true) }))
	n.Dispose()

	time.Sleep(30 * time.Millisecond)
	if fired.Load() {
		t.Error("watchdog should not fire after dispose")
	}
}

func TestChain_AppliesLeftToRight(t *testing.T) {
	var order []string
	mwA := func(n nerve.Nerve) nerve.Nerve {
		order = append(order, "A")
		return n
	}
	mwB := func(n nerve.Nerve) nerve.Nerve {
		order = append(order, "B")
		return n
	}

	n := nerve.New()
	defer n.Dispose()
	chained := nerve.Chain(mwA, mwB)
	chained(n.AsNerve())

	if len(order) != 2 || order[0] != "B" || order[1] != "A" {
		t.Errorf("expected [B A] (inner-first wrapping); got %v", order)
	}
}
