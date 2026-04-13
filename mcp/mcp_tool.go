package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

var _ tool.Tool = (*mcpTool)(nil)

// mcpTool implements tool.Tool by delegating Execute to an MCP server via tools/call.
type mcpTool struct {
	serverName  string // prefix for namespacing
	name        string // the MCP tool name (unprefixed)
	description string
	schema      json.RawMessage
	session     *sdkmcp.ClientSession
}

func (t *mcpTool) Name() string                 { return t.serverName + "." + t.name }
func (t *mcpTool) Description() string          { return t.description }
func (t *mcpTool) InputSchema() json.RawMessage { return t.schema }

func (t *mcpTool) Execute(ctx context.Context, input json.RawMessage) (string, error) {
	// Convert json.RawMessage to the any type expected by CallToolParams.Arguments.
	var args any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("mcp tool %s: unmarshal input: %w", t.Name(), err)
		}
	}

	result, err := t.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      t.name,
		Arguments: args,
	})
	if err != nil {
		return "", fmt.Errorf("mcp tool %s: call: %w", t.Name(), err)
	}

	if result.IsError {
		text := extractText(result)
		return "", fmt.Errorf("%w: %s: %s", ErrMCPToolError, t.Name(), text)
	}

	return extractText(result), nil
}

// extractText concatenates all TextContent blocks from a CallToolResult.
func extractText(result *sdkmcp.CallToolResult) string {
	var parts []string
	for _, c := range result.Content {
		if tc, ok := c.(*sdkmcp.TextContent); ok {
			parts = append(parts, tc.Text)
		}
	}
	return strings.Join(parts, "\n")
}
