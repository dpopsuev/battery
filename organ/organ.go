// Package organ defines the EDA agent component contract.
// An Organ is a pluggable component that contributes tools, handles events,
// and can inject directives and context assembly handlers.
package organ

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/battery/nerve"
	"github.com/dpopsuev/battery/tool"
)

// Organ is the EDA agent component interface.
type Organ interface {
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

// Subscriptions declares which event types this organ handles.
type Subscriptions struct {
	Motor []string
	Sense []string
}

// Contributions holds cross-organ contribution points.
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
