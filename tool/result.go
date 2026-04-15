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

// wireContent is the JSON representation with a type discriminator.
type wireContent struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	MIMEType string `json:"mimeType,omitempty"`
	Data     []byte `json:"data,omitempty"`
	URI      string `json:"uri,omitempty"`
	Name     string `json:"name,omitempty"`
	Desc     string `json:"description,omitempty"`
	Blob     []byte `json:"blob,omitempty"`
}

// MarshalJSON serializes Result with type-discriminated Content.
func (r Result) MarshalJSON() ([]byte, error) {
	type resultWire struct {
		Content           []wireContent `json:"content"`
		StructuredContent any           `json:"structuredContent,omitempty"`
		IsError           bool          `json:"isError,omitempty"`
	}
	w := resultWire{StructuredContent: r.StructuredContent, IsError: r.IsError}
	for _, c := range r.Content {
		switch v := c.(type) {
		case TextContent:
			w.Content = append(w.Content, wireContent{Type: "text", Text: v.Text})
		case ImageContent:
			w.Content = append(w.Content, wireContent{Type: "image", MIMEType: v.MIMEType, Data: v.Data})
		case AudioContent:
			w.Content = append(w.Content, wireContent{Type: "audio", MIMEType: v.MIMEType, Data: v.Data})
		case ResourceLink:
			w.Content = append(w.Content, wireContent{Type: "resource_link", URI: v.URI, Name: v.Name, Desc: v.Description, MIMEType: v.MIMEType})
		case ResourceContent:
			w.Content = append(w.Content, wireContent{Type: "resource", URI: v.URI, MIMEType: v.MIMEType, Text: v.Text, Blob: v.Blob})
		}
	}
	return json.Marshal(w)
}

// UnmarshalJSON deserializes Result with type-discriminated Content.
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
		wc := &w.Content[i]
		switch wc.Type {
		case "text":
			r.Content = append(r.Content, TextContent{Text: wc.Text})
		case "image":
			r.Content = append(r.Content, ImageContent{MIMEType: wc.MIMEType, Data: wc.Data})
		case "audio":
			r.Content = append(r.Content, AudioContent{MIMEType: wc.MIMEType, Data: wc.Data})
		case "resource_link":
			r.Content = append(r.Content, ResourceLink{URI: wc.URI, Name: wc.Name, Description: wc.Desc, MIMEType: wc.MIMEType})
		case "resource":
			r.Content = append(r.Content, ResourceContent{URI: wc.URI, MIMEType: wc.MIMEType, Text: wc.Text, Blob: wc.Blob})
		}
	}
	return nil
}
