package mcp

import (
	"encoding/json"
	"errors"
	"fmt"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TypeHints maps JSON field names to human-readable type descriptions.
// Used by UnmarshalWithHints to produce actionable error messages when
// the LLM sends the wrong type (e.g., a string instead of an array).
type TypeHints map[string]string

// UnmarshalWithHints unmarshals raw JSON into target, returning a
// CallToolResult with IsError=true on failure. When a UnmarshalTypeError
// matches a hint, the error message includes the expected format.
// Returns nil on success.
func UnmarshalWithHints[In any](raw json.RawMessage, target *In, hints TypeHints) *sdkmcp.CallToolResult {
	if err := json.Unmarshal(raw, target); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && hints != nil {
			if hint, ok := hints[typeErr.Field]; ok {
				return errorResult(fmt.Sprintf("field %q must be %s (got JSON %s)", typeErr.Field, hint, typeErr.Value))
			}
		}
		return errorResult("invalid arguments: " + err.Error())
	}
	return nil
}

func errorResult(msg string) *sdkmcp.CallToolResult {
	return &sdkmcp.CallToolResult{
		IsError: true,
		Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: msg}},
	}
}
