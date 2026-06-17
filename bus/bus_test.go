package bus_test

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dpopsuev/battery/bus"
)

func motor(typ string, payload any) bus.MotorEvent {
	raw, _ := json.Marshal(payload)
	return bus.MotorEvent{
		Event:   bus.Event{Type: typ, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: raw,
	}
}

func sense(typ string, payload any, isFinal bool) bus.SenseEvent {
	raw, _ := json.Marshal(payload)
	return bus.SenseEvent{
		Event:   bus.Event{Type: typ, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: raw,
		IsFinal: isFinal,
	}
}

func TestInProcessBus_TypeRoutedDispatch(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var received []string

	b.Subscribe("fs.read", func(_ bus.MotorEvent) { received = append(received, "fs.read") })
	b.Subscribe("shell.exec", func(_ bus.MotorEvent) { received = append(received, "shell.exec") })

	b.Publish(motor("fs.read", map[string]string{"path": "/tmp"}))
	b.Publish(motor("shell.exec", map[string]string{"cmd": "ls"}))
	b.Publish(motor("fs.read", map[string]string{"path": "/etc"}))

	if len(received) != 3 {
		t.Fatalf("expected 3 dispatches; got %d: %v", len(received), received)
	}
	if received[0] != "fs.read" || received[1] != "shell.exec" || received[2] != "fs.read" {
		t.Errorf("wrong dispatch order: %v", received)
	}
}

func TestInProcessBus_WildcardSubscriber(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var count int

	b.Subscribe("*", func(_ bus.MotorEvent) { count++ })
	b.Publish(motor("a", nil))
	b.Publish(motor("b", nil))
	b.Publish(motor("c", nil))

	if count != 3 {
		t.Errorf("wildcard should receive all 3 events; got %d", count)
	}
}

func TestInProcessBus_Unsubscribe(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var count int

	unsub := b.Subscribe("x", func(_ bus.MotorEvent) { count++ })
	b.Publish(motor("x", nil))
	unsub()
	b.Publish(motor("x", nil))

	if count != 1 {
		t.Errorf("should receive 1 event before unsub; got %d", count)
	}
}

func TestInProcessBus_DeadLetter(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var dead []string

	b.SetDeadLetter(func(e bus.MotorEvent) { dead = append(dead, e.Type) })
	b.Publish(motor("unhandled", nil))

	if len(dead) != 1 || dead[0] != "unhandled" {
		t.Errorf("dead letter should capture unhandled event; got %v", dead)
	}
}

func TestInProcessBus_DeadLetterNotFiredWhenHandled(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var deadCount int

	b.SetDeadLetter(func(_ bus.MotorEvent) { deadCount++ })
	b.Subscribe("handled", func(_ bus.MotorEvent) {})
	b.Publish(motor("handled", nil))

	if deadCount != 0 {
		t.Errorf("dead letter should not fire for handled events; got %d", deadCount)
	}
}

func TestInProcessBus_MultipleSubscribersSameType(t *testing.T) {
	b := bus.NewInProcessBus[bus.SenseEvent]()
	var a, c int

	b.Subscribe("result", func(_ bus.SenseEvent) { a++ })
	b.Subscribe("result", func(_ bus.SenseEvent) { c++ })
	b.Publish(sense("result", nil, true))

	if a != 1 || c != 1 {
		t.Errorf("both subscribers should fire; got a=%d b=%d", a, c)
	}
}

func TestInProcessBus_ConcurrentSafety(t *testing.T) {
	b := bus.NewInProcessBus[bus.MotorEvent]()
	var count atomic.Int64

	b.Subscribe("ping", func(_ bus.MotorEvent) { count.Add(1) })

	var wg sync.WaitGroup
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			b.Publish(motor("ping", i))
		}(i)
	}
	wg.Wait()

	if count.Load() != 100 {
		t.Errorf("expected 100 concurrent dispatches; got %d", count.Load())
	}
}

func TestCorrelationTracker_StampsElapsed(t *testing.T) {
	ct := bus.NewCorrelationTracker()

	e1 := bus.Event{Type: "a", CorrelationID: "c1"}
	ct.Stamp(&e1)
	if e1.Elapsed != 0 {
		t.Errorf("first stamp should have 0 elapsed; got %v", e1.Elapsed)
	}

	time.Sleep(5 * time.Millisecond)

	e2 := bus.Event{Type: "b", CorrelationID: "c1"}
	ct.Stamp(&e2)
	if e2.Elapsed < 4*time.Millisecond {
		t.Errorf("second stamp should have >4ms elapsed; got %v", e2.Elapsed)
	}
}

func TestCorrelationTracker_EvictsCleanly(t *testing.T) {
	ct := bus.NewCorrelationTracker()

	e1 := bus.Event{Type: "a", CorrelationID: "c1"}
	ct.Stamp(&e1)
	ct.Evict("c1")

	e2 := bus.Event{Type: "b", CorrelationID: "c1"}
	ct.Stamp(&e2)
	if e2.Elapsed != 0 {
		t.Errorf("after evict, elapsed should reset to 0; got %v", e2.Elapsed)
	}
}
