// Package typed provides generic tool adapters that derive JSON Schema
// from Go struct types, eliminating hand-written schema definitions.
package typed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dpopsuev/battery/tool"
	"github.com/google/jsonschema-go/jsonschema"
)

// SchemaFor derives a JSON Schema from Go struct type T.
// Panics if the type cannot be represented as JSON Schema.
func SchemaFor[T any]() json.RawMessage {
	s, err := SchemaForSafe[T]()
	if err != nil {
		panic("battery/typed: " + err.Error())
	}
	return s
}

// SchemaForSafe derives a JSON Schema from Go struct type T,
// returning an error if the type is not representable.
func SchemaForSafe[T any]() (json.RawMessage, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, fmt.Errorf("schema for %T: %w", *new(T), err)
	}
	data, err := json.Marshal(schema)
	if err != nil {
		return nil, fmt.Errorf("marshal schema: %w", err)
	}
	return data, nil
}

// TypedTool adapts a typed handler into a tool.Tool.
// The InputSchema is derived automatically from the In type parameter.
type TypedTool[In any] struct {
	name        string
	description string
	schema      json.RawMessage
	handler     func(ctx context.Context, input In) (string, error)
}

var _ tool.Tool = (*TypedTool[struct{}])(nil)

// New creates a TypedTool with schema derived from In.
func New[In any](name, description string, handler func(context.Context, In) (string, error)) *TypedTool[In] {
	return &TypedTool[In]{
		name:        name,
		description: description,
		schema:      SchemaFor[In](),
		handler:     handler,
	}
}

// Name returns the tool name.
func (t *TypedTool[In]) Name() string { return t.name }

// Description returns the tool description.
func (t *TypedTool[In]) Description() string { return t.description }

// InputSchema returns the auto-derived JSON Schema for the input type.
func (t *TypedTool[In]) InputSchema() json.RawMessage { return t.schema }

// Execute unmarshals raw JSON into the typed input and calls the handler.
func (t *TypedTool[In]) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	var in In
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return "", fmt.Errorf("battery: typed tool %s: unmarshal input: %w", t.name, err)
		}
	}
	return t.handler(ctx, in)
}
