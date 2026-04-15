package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Result is the structured output of a tool execution.
// Replaces the former (string, error) return — carries multiple content
// blocks, structured JSON output, and an error flag.
type Result struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Text concatenates all TextContent blocks, separated by newlines.
// Returns empty string if no TextContent blocks exist.
func (r Result) Text() string {
	var parts []string
	for _, c := range r.Content {
		if tc, ok := c.(TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// TextResult creates a Result with a single TextContent block.
func TextResult(s string) Result {
	return Result{
		Content: []Content{TextContent{Text: s}},
	}
}

// ErrorResult creates a Result with an error message and IsError=true.
func ErrorResult(err error) Result {
	return Result{
		Content: []Content{TextContent{Text: err.Error()}},
		IsError: true,
	}
}

// StructuredResult creates a Result with StructuredContent and a TextContent
// fallback containing the JSON representation.
func StructuredResult(v any) (Result, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return Result{}, fmt.Errorf("battery: marshal structured result: %w", err)
	}
	return Result{
		Content:           []Content{TextContent{Text: string(data)}},
		StructuredContent: json.RawMessage(data),
	}, nil
}
