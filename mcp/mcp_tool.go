package mcp

import (
	"context"
	"encoding/json"
	"fmt"

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

func (t *mcpTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var args any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &args); err != nil {
			return tool.Result{}, fmt.Errorf("mcp tool %s: unmarshal input: %w", t.Name(), err)
		}
	}

	sdkResult, err := t.session.CallTool(ctx, &sdkmcp.CallToolParams{
		Name:      t.name,
		Arguments: args,
	})
	if err != nil {
		return tool.Result{}, fmt.Errorf("mcp tool %s: call: %w", t.Name(), err)
	}

	// No translation needed — tool.Content IS sdkmcp.Content.
	return tool.ResultFromSDK(sdkResult), nil
}
