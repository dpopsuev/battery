package tool

import (
	"encoding/json"
	"fmt"
	"strings"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// Result is the structured output of a tool execution.
// Uses SDK content types directly — no translation layer.
type Result struct {
	Content           []Content `json:"content"`
	StructuredContent any       `json:"structuredContent,omitempty"`
	IsError           bool      `json:"isError,omitempty"`
}

// Text concatenates all TextContent blocks, separated by newlines.
func (r Result) Text() string {
	var parts []string
	for _, c := range r.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}

// TextResult creates a Result with a single TextContent block.
func TextResult(s string) Result {
	return Result{
		Content: []Content{&sdkmcp.TextContent{Text: s}},
	}
}

// ErrorResult creates a Result with an error message and IsError=true.
func ErrorResult(err error) Result {
	return Result{
		Content: []Content{&sdkmcp.TextContent{Text: err.Error()}},
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
		Content:           []Content{&sdkmcp.TextContent{Text: string(data)}},
		StructuredContent: json.RawMessage(data),
	}, nil
}

// ToSDK converts a Battery Result to an SDK CallToolResult.
func (r Result) ToSDK() *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		Content:           r.Content,
		StructuredContent: r.StructuredContent,
		IsError:           r.IsError,
	}
}

// ResultFromSDK converts an SDK CallToolResult to a Battery Result.
func ResultFromSDK(sdk *sdkmcp.CallToolResult) Result {
	return Result{
		Content:           sdk.Content,
		StructuredContent: sdk.StructuredContent,
		IsError:           sdk.IsError,
	}
}
