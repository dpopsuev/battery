// Package server provides tool presentation metadata — enriched descriptions,
// keywords, categories, and intent-based triage for tool discovery.
package server

import (
	"context"
	"encoding/json"
)

// Handler is a string-returning tool handler (convenience for simple tools).
type Handler func(ctx context.Context, input json.RawMessage) (string, error)

// JSONResult marshals v as indented JSON. Convenience for Handler implementations.
func JSONResult(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToolMeta is enriched metadata beyond tool.Tool — keywords, categories, priority.
type ToolMeta struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Keywords    []string          `json:"keywords,omitempty"`
	Categories  []string          `json:"categories,omitempty"`
	Priority    int               `json:"priority,omitempty"`
	DefaultArgs map[string]any    `json:"default_args,omitempty"`
	Rationale   map[string]string `json:"rationale,omitempty"`
	InputSchema  json.RawMessage   `json:"input_schema,omitempty"`
	OutputSchema json.RawMessage   `json:"output_schema,omitempty"`
}
