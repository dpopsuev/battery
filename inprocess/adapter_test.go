package inprocess_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/inprocess"
	"github.com/dpopsuev/battery/adapter"
	"github.com/dpopsuev/battery/tool"
)

const pingEvent = "echo.ping"

type pingTool struct{}

func (pingTool) Name() string               { return pingEvent }
func (pingTool) Description() string         { return "Ping test tool." }
func (pingTool) InputSchema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (pingTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.TextResult("pong"), nil
}

func echoAdapter() adapter.EventAdapter {
	return adapter.NewBuilder("echo").
		WithDescription("Echo adapter for testing.").
		MotorAction(pingEvent, pingTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{
				Payload: json.RawMessage(`{"reply":"pong"}`),
				IsFinal: true,
			}, nil
		}).
		Build()
}

func TestAdapter_TwoOrgansCompose(t *testing.T) {
	a := inprocess.New()
	a.Load(echoAdapter())

	upperOrgan := adapter.NewBuilder("upper").
		WithDescription("Uppercases echo results.").
		MotorAction("upper.shout", pingTool{}, func(_ context.Context, _ bus.MotorEvent) (bus.SenseEvent, error) {
			return bus.SenseEvent{
				Payload: json.RawMessage(`{"reply":"PONG"}`),
				IsFinal: true,
			}, nil
		}).
		Build()
	a.Load(upperOrgan)

	if err := a.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := a.Stop(context.Background()); err != nil {
			t.Fatal(err)
		}
	}()

	var echoResult, upperResult bus.SenseEvent
	a.SubscribeSense(pingEvent, func(e bus.SenseEvent) { echoResult = e })
	a.SubscribeSense("upper.shout", func(e bus.SenseEvent) { upperResult = e })

	a.PublishMotor(bus.MotorEvent{
		Event:   bus.Event{Type: pingEvent, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})
	a.PublishMotor(bus.MotorEvent{
		Event:   bus.Event{Type: "upper.shout", CorrelationID: "c2", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})

	if string(echoResult.Payload) != `{"reply":"pong"}` {
		t.Errorf("echo result = %s; want pong", echoResult.Payload)
	}
	if string(upperResult.Payload) != `{"reply":"PONG"}` {
		t.Errorf("upper result = %s; want PONG", upperResult.Payload)
	}
}

func TestAdapter_StopUnmountsInReverse(t *testing.T) {
	a := inprocess.New()
	a.Load(echoAdapter())

	if err := a.Start(context.Background()); err != nil {
		t.Fatal(err)
	}

	var count int
	a.SubscribeSense(pingEvent, func(_ bus.SenseEvent) { count++ })

	a.PublishMotor(bus.MotorEvent{
		Event:   bus.Event{Type: pingEvent, CorrelationID: "c1", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})

	if err := a.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}

	a.PublishMotor(bus.MotorEvent{
		Event:   bus.Event{Type: pingEvent, CorrelationID: "c2", Timestamp: time.Now()},
		Payload: json.RawMessage(`{}`),
	})

	if count != 1 {
		t.Errorf("expected 1 sense event before stop; got %d", count)
	}
}

func TestAdapter_DoubleStartErrors(t *testing.T) {
	a := inprocess.New()
	a.Load(echoAdapter())
	_ = a.Start(context.Background())
	defer func() { _ = a.Stop(context.Background()) }()

	err := a.Start(context.Background())
	if err == nil {
		t.Error("double start should error")
	}
}
