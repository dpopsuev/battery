package adapter_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/adapter"
	"github.com/dpopsuev/battery/tool"
)

const echoEvent = "echo.ping"

type echoTool struct{}

func (echoTool) Name() string                    { return echoEvent }
func (echoTool) Description() string              { return "Echo a message back." }
func (echoTool) InputSchema() json.RawMessage      { return json.RawMessage(`{"type":"object"}`) }
func (echoTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.TextResult("pong"), nil
}

func TestBuilder_ConstructsValidOrgan(t *testing.T) {
	o := adapter.NewBuilder("echo").
		WithDescription("Echo organ").
		WithLabels("test", "echo").
		WithDirectives("Use echo.ping to test.").
		MotorAction(echoEvent, echoTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{Payload: json.RawMessage(`{"reply":"pong"}`)}, nil
		}).
		Build()

	if o.Name() != "echo" {
		t.Errorf("name = %q; want echo", o.Name())
	}
	if o.Description() != "Echo organ" {
		t.Errorf("description = %q; want 'Echo organ'", o.Description())
	}
	if len(o.Tools()) != 1 {
		t.Errorf("tools = %d; want 1", len(o.Tools()))
	}
	if len(o.Subscriptions().Motor) != 1 || o.Subscriptions().Motor[0] != echoEvent {
		t.Errorf("subscriptions.motor = %v; want [echo.ping]", o.Subscriptions().Motor)
	}
	if len(o.Directives()) != 1 {
		t.Errorf("directives = %d; want 1", len(o.Directives()))
	}
}

func TestBuilder_MountWiresMotorToSense(t *testing.T) {
	o := adapter.NewBuilder("echo").
		MotorAction(echoEvent, echoTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{
				Payload: json.RawMessage(`{"reply":"pong"}`),
				IsFinal: true,
			}, nil
		}).
		Build()

	n := nerve.New()
	defer n.Dispose()

	unmount := o.Mount(n.AsNerve())
	defer unmount()

	var received bus.SenseEvent
	n.Sense().Subscribe(echoEvent, func(e bus.SenseEvent) { received = e })

	n.Motor().Publish(bus.MotorEvent{
		Event:   bus.Event{Type: echoEvent, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{"message":"hello"}`),
	})

	if received.CorrelationID != "c1" {
		t.Errorf("correlationID = %q; want c1", received.CorrelationID)
	}
	if !received.IsFinal {
		t.Error("expected isFinal=true")
	}
	if string(received.Payload) != `{"reply":"pong"}` {
		t.Errorf("payload = %s; want {\"reply\":\"pong\"}", received.Payload)
	}
}

func TestBuilder_MountHandlerErrorPublishesErrorSense(t *testing.T) {
	o := adapter.NewBuilder("fail").
		MotorAction("fail.op", echoTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{}, context.DeadlineExceeded
		}).
		Build()

	n := nerve.New()
	defer n.Dispose()

	unmount := o.Mount(n.AsNerve())
	defer unmount()

	var received bus.SenseEvent
	n.Sense().Subscribe("fail.op", func(e bus.SenseEvent) { received = e })

	n.Motor().Publish(bus.MotorEvent{
		Event:   bus.Event{Type: "fail.op", CorrelationID: "c2", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})

	if !received.IsError {
		t.Error("expected isError=true on handler failure")
	}
	if received.ErrorMessage == "" {
		t.Error("expected non-empty errorMessage")
	}
}

func TestBuilder_UnmountCleansUp(t *testing.T) {
	o := adapter.NewBuilder("echo").
		MotorAction(echoEvent, echoTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{Payload: json.RawMessage(`{}`)}, nil
		}).
		Build()

	n := nerve.New()
	defer n.Dispose()

	unmount := o.Mount(n.AsNerve())

	var count int
	n.Sense().Subscribe(echoEvent, func(_ bus.SenseEvent) { count++ })

	n.Motor().Publish(bus.MotorEvent{
		Event: bus.Event{Type: echoEvent, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})
	unmount()
	n.Motor().Publish(bus.MotorEvent{
		Event: bus.Event{Type: echoEvent, CorrelationID: "c2", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})

	if count != 1 {
		t.Errorf("expected 1 sense event (before unmount); got %d", count)
	}
}

func TestContextAssemblyHandler_Signature(t *testing.T) {
	handler := adapter.ContextAssemblyHandler(func(_ context.Context, input adapter.ContextAssemblyInput) (adapter.ContextAssemblyOutput, error) {
		if input.Turn < 1 {
			return adapter.ContextAssemblyOutput{Abort: true}, nil
		}
		return adapter.ContextAssemblyOutput{}, nil
	})

	out, err := handler(context.Background(), adapter.ContextAssemblyInput{Turn: 0})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Abort {
		t.Error("expected abort on turn 0")
	}

	out, err = handler(context.Background(), adapter.ContextAssemblyInput{Turn: 1})
	if err != nil {
		t.Fatal(err)
	}
	if out.Abort {
		t.Error("should not abort on turn 1")
	}
}
