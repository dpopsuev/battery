package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Registry holds registered tools and dispatches calls by name.
type Registry struct {
	tools map[string]Tool
}

var _ Executor = (*Registry)(nil)

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Unregister removes a tool from the registry by name.
func (r *Registry) Unregister(name string) {
	delete(r.tools, name)
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, error) {
	t, ok := r.tools[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return t, nil
}

// Execute dispatches a tool call by name.
// Returns ErrNotFound if the tool is not registered or not available.
func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	t, err := r.Get(name)
	if err != nil {
		return Result{}, err
	}
	if !isAvailable(t) {
		return Result{}, fmt.Errorf("%w: %s", ErrNotFound, name)
	}
	return t.Execute(ctx, input)
}

// All returns all registered tools that are currently available.
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		if isAvailable(t) {
			out = append(out, t)
		}
	}
	return out
}

// Names returns available tool names sorted alphabetically.
func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.tools))
	for name, t := range r.tools {
		if isAvailable(t) {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

// isAvailable checks the Availability optional interface.
// Tools that do not implement Availability are always available.
func isAvailable(t Tool) bool {
	if a, ok := t.(Availability); ok {
		return a.Available()
	}
	return true
}
