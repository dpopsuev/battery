package tool

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Result is the structured output of a tool execution.
// Transport-agnostic — uses Battery-owned content types.
type Result struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Text concatenates all TextContent blocks, separated by newlines.
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

// MarshalJSON serializes Result using the content type registry.
func (r Result) MarshalJSON() ([]byte, error) {
	type resultWire struct {
		Content           []wireContent `json:"content"`
		StructuredContent any           `json:"structuredContent,omitempty"`
		IsError           bool          `json:"isError,omitempty"`
	}
	w := resultWire{StructuredContent: r.StructuredContent, IsError: r.IsError}
	for _, c := range r.Content {
		if wc, ok := encodeContent(c); ok {
			w.Content = append(w.Content, wc)
		}
	}
	return json.Marshal(w)
}

// UnmarshalJSON deserializes Result using the content type registry.
func (r *Result) UnmarshalJSON(data []byte) error {
	type resultWire struct {
		Content           []wireContent   `json:"content"`
		StructuredContent json.RawMessage `json:"structuredContent,omitempty"`
		IsError           bool            `json:"isError,omitempty"`
	}
	var w resultWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	r.IsError = w.IsError
	if len(w.StructuredContent) > 0 {
		r.StructuredContent = w.StructuredContent
	}
	r.Content = make([]Content, 0, len(w.Content))
	for i := range w.Content {
		if c, ok := decodeContent(w.Content[i]); ok {
			r.Content = append(r.Content, c)
		}
	}
	return nil
}
