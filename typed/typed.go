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
// InputSchema is derived from In, OutputSchema from Out.
// The handler returns (Out, error) — Out is automatically marshaled
// into Result.StructuredContent with a TextContent JSON fallback.
type TypedTool[In, Out any] struct {
	name         string
	description  string
	inputSchema  json.RawMessage
	outputSchema json.RawMessage
	handler      func(ctx context.Context, input In) (Out, error)
}

var _ tool.Tool = (*TypedTool[struct{}, struct{}])(nil)
var _ tool.Cacheable = (*TypedTool[struct{}, struct{}])(nil)

// New creates a TypedTool with schemas derived from In and Out.
func New[In, Out any](name, description string, handler func(context.Context, In) (Out, error)) *TypedTool[In, Out] {
	return &TypedTool[In, Out]{
		name:         name,
		description:  description,
		inputSchema:  SchemaFor[In](),
		outputSchema: SchemaFor[Out](),
		handler:      handler,
	}
}

// Name returns the tool name.
func (t *TypedTool[In, Out]) Name() string { return t.name }

// Description returns the tool description.
func (t *TypedTool[In, Out]) Description() string { return t.description }

// InputSchema returns the auto-derived JSON Schema for the input type.
func (t *TypedTool[In, Out]) InputSchema() json.RawMessage { return t.inputSchema }

// OutputSchema returns the auto-derived JSON Schema for the output type.
func (t *TypedTool[In, Out]) OutputSchema() json.RawMessage { return t.outputSchema }

// CacheKey returns a deterministic key derived from the canonical JSON
// serialization of the typed input. Stateless typed tools get caching for free.
func (t *TypedTool[In, Out]) CacheKey(_ context.Context, input json.RawMessage) (string, bool) {
	if len(input) == 0 {
		return t.name + ":{}", true
	}
	// Canonical form: unmarshal → re-marshal for deterministic key ordering.
	var canonical any
	if err := json.Unmarshal(input, &canonical); err != nil {
		return "", false
	}
	data, err := json.Marshal(canonical)
	if err != nil {
		return "", false
	}
	return t.name + ":" + string(data), true
}

// Execute unmarshals raw JSON into In, calls the handler, and marshals
// the Out value into Result.StructuredContent with a TextContent fallback.
func (t *TypedTool[In, Out]) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in In
	if len(input) > 0 {
		if err := json.Unmarshal(input, &in); err != nil {
			return tool.Result{}, fmt.Errorf("battery: typed tool %s: unmarshal input: %w", t.name, err)
		}
	}
	out, err := t.handler(ctx, in)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.StructuredResult(out)
}
