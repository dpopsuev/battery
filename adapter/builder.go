package adapter

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/battery/bus"
	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/tool"
)

// MotorHandler handles a Motor event and returns a Sense result.
type MotorHandler func(ctx context.Context, event bus.MotorEvent) (bus.SenseEvent, error)

type motorAction struct {
	eventType string
	tool      tool.Tool
	handler   MotorHandler
}

// Builder constructs an EventAdapter from action maps.
type Builder struct {
	name          string
	description   string
	labels        []string
	directives    []string
	motorActions  []motorAction
	contributions Contributions
}

// NewBuilder starts building an adapter with the given name.
func NewBuilder(name string) *Builder {
	return &Builder{name: name}
}

// WithDescription sets the adapter description.
func (b *Builder) WithDescription(d string) *Builder {
	b.description = d
	return b
}

// WithLabels sets the adapter labels.
func (b *Builder) WithLabels(labels ...string) *Builder {
	b.labels = labels
	return b
}

// WithDirectives sets directives injected into the system prompt.
func (b *Builder) WithDirectives(directives ...string) *Builder {
	b.directives = directives
	return b
}

// WithContributions sets cross-adapter contributions.
func (b *Builder) WithContributions(c Contributions) *Builder {
	b.contributions = c
	return b
}

// MotorAction registers a tool and its Motor event handler.
func (b *Builder) MotorAction(eventType string, t tool.Tool, handler MotorHandler) *Builder {
	b.motorActions = append(b.motorActions, motorAction{eventType: eventType, tool: t, handler: handler})
	return b
}

// Build constructs the adapter.
func (b *Builder) Build() EventAdapter {
	tools := make([]tool.Tool, 0, len(b.motorActions))
	motorSubs := make([]string, 0, len(b.motorActions))
	for _, a := range b.motorActions {
		tools = append(tools, a.tool)
		motorSubs = append(motorSubs, a.eventType)
	}
	return &builtEventAdapter{
		name:          b.name,
		description:   b.description,
		labels:        b.labels,
		directives:    b.directives,
		tools:         tools,
		motorActions:  b.motorActions,
		subscriptions: Subscriptions{Motor: motorSubs},
		contributions: b.contributions,
	}
}

type builtEventAdapter struct {
	name          string
	description   string
	labels        []string
	directives    []string
	tools         []tool.Tool
	motorActions  []motorAction
	subscriptions Subscriptions
	contributions Contributions
}

func (o *builtEventAdapter) Name() string             { return o.name }
func (o *builtEventAdapter) Description() string      { return o.description }
func (o *builtEventAdapter) Labels() []string         { return o.labels }
func (o *builtEventAdapter) Tools() []tool.Tool       { return o.tools }
func (o *builtEventAdapter) Close() error             { return nil }
func (o *builtEventAdapter) Subscriptions() Subscriptions { return o.subscriptions }
func (o *builtEventAdapter) Directives() []string     { return o.directives }
func (o *builtEventAdapter) Contributions() Contributions { return o.contributions }

func (o *builtEventAdapter) Mount(n nerve.Nerve) func() {
	unsubs := make([]func(), 0, len(o.motorActions))
	for _, a := range o.motorActions {
		a := a
		unsub := n.Motor().Subscribe(a.eventType, func(event bus.MotorEvent) {
			result, err := a.handler(context.Background(), event)
			if err != nil {
				errPayload, _ := json.Marshal(map[string]string{"error": err.Error()})
				n.Sense().Publish(bus.SenseEvent{
					Event:        bus.Event{Type: a.eventType, CorrelationID: event.CorrelationID},
					Payload:      errPayload,
					IsError:      true,
					ErrorMessage: err.Error(),
					IsFinal:      true,
				})
				return
			}
			result.Type = a.eventType
			result.CorrelationID = event.CorrelationID
			if !result.IsFinal {
				result.IsFinal = true
			}
			n.Sense().Publish(result)
		})
		unsubs = append(unsubs, unsub)
	}
	return func() {
		for _, u := range unsubs {
			u()
		}
	}
}
