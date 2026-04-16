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

	return resultFromSDK(sdkResult), nil
}

// sdkContentConverter converts an SDK Content to a Battery Content.
type sdkContentConverter func(sdkmcp.Content) (tool.Content, bool)

// sdkConverters maps SDK content types to Battery converters.
var sdkConverters []sdkContentConverter //nolint:gochecknoglobals // strategy registry

func init() {
	sdkConverters = []sdkContentConverter{
		func(c sdkmcp.Content) (tool.Content, bool) {
			if v, ok := c.(*sdkmcp.TextContent); ok {
				return tool.TextContent{Text: v.Text}, true
			}
			return nil, false
		},
		func(c sdkmcp.Content) (tool.Content, bool) {
			if v, ok := c.(*sdkmcp.ImageContent); ok {
				return tool.ImageContent{MIMEType: v.MIMEType, Data: v.Data}, true
			}
			return nil, false
		},
		func(c sdkmcp.Content) (tool.Content, bool) {
			if v, ok := c.(*sdkmcp.AudioContent); ok {
				return tool.AudioContent{MIMEType: v.MIMEType, Data: v.Data}, true
			}
			return nil, false
		},
		func(c sdkmcp.Content) (tool.Content, bool) {
			if v, ok := c.(*sdkmcp.ResourceLink); ok {
				return tool.ResourceLink{URI: v.URI, Name: v.Name, Description: v.Description, MIMEType: v.MIMEType}, true
			}
			return nil, false
		},
		func(c sdkmcp.Content) (tool.Content, bool) {
			if v, ok := c.(*sdkmcp.EmbeddedResource); ok && v.Resource != nil {
				return tool.ResourceContent{URI: v.Resource.URI, MIMEType: v.Resource.MIMEType, Text: v.Resource.Text, Blob: v.Resource.Blob}, true
			}
			return nil, false
		},
	}
}

// resultFromSDK translates an SDK CallToolResult to a Battery Result.
func resultFromSDK(sdk *sdkmcp.CallToolResult) tool.Result {
	r := tool.Result{
		IsError:           sdk.IsError,
		StructuredContent: sdk.StructuredContent,
	}
	for _, sc := range sdk.Content {
		for _, conv := range sdkConverters {
			if bc, ok := conv(sc); ok {
				r.Content = append(r.Content, bc)
				break
			}
		}
	}
	return r
}
