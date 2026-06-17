// Package adapter defines the EDA event adapter contract.
// An EventAdapter is a pluggable component that bridges capabilities
// into the event bus system — handling Motor commands, publishing
// Sense observations, and contributing context assembly handlers.
package adapter

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/tool"
)

// EventAdapter is the EDA component interface.
type EventAdapter interface {
	Name() string
	Description() string
	Labels() []string
	Tools() []tool.Tool
	Mount(n nerve.Nerve) (unmount func())
	Close() error
	Subscriptions() Subscriptions
	Directives() []string
	Contributions() Contributions
}

// Subscriptions declares which event types this adapter handles.
type Subscriptions struct {
	Motor []string
	Sense []string
}

// Contributions holds cross-adapter contribution points.
type Contributions struct {
	ContextAssemble ContextAssemblyHandler
}

// ContextAssemblyInput is the input to a context.assemble pipeline stage.
type ContextAssemblyInput struct {
	Messages []json.RawMessage
	Tools    []tool.Tool
	Turn     int
}

// ContextAssemblyOutput is the output from a context.assemble contributor.
type ContextAssemblyOutput struct {
	Messages []json.RawMessage
	Tools    []tool.Tool
	Skip     bool
	Reply    string
	Abort    bool
}

// ContextAssemblyHandler is a single stage in the context.assemble pipeline.
type ContextAssemblyHandler func(ctx context.Context, input ContextAssemblyInput) (ContextAssemblyOutput, error)
