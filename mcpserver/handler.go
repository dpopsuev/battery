package mcpserver

import (
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// toSDKResult translates a Battery tool.Result to an MCP CallToolResult,
// preserving all content types and structured content.
func toSDKResult(r tool.Result) *sdkmcp.CallToolResult {
	sdk := &sdkmcp.CallToolResult{
		IsError:           r.IsError,
		StructuredContent: r.StructuredContent,
	}

	for _, c := range r.Content {
		switch v := c.(type) {
		case tool.TextContent:
			sdk.Content = append(sdk.Content, &sdkmcp.TextContent{Text: v.Text})
		case tool.ImageContent:
			sdk.Content = append(sdk.Content, &sdkmcp.ImageContent{MIMEType: v.MIMEType, Data: v.Data})
		case tool.AudioContent:
			sdk.Content = append(sdk.Content, &sdkmcp.AudioContent{MIMEType: v.MIMEType, Data: v.Data})
		case tool.ResourceLink:
			sdk.Content = append(sdk.Content, &sdkmcp.ResourceLink{
				URI: v.URI, Name: v.Name, Description: v.Description, MIMEType: v.MIMEType,
			})
		case tool.ResourceContent:
			sdk.Content = append(sdk.Content, &sdkmcp.EmbeddedResource{
				Resource: &sdkmcp.ResourceContents{
					URI: v.URI, MIMEType: v.MIMEType, Text: v.Text, Blob: v.Blob,
				},
			})
		}
	}

	// Ensure Content is never nil (MCP requires it).
	if sdk.Content == nil {
		sdk.Content = []sdkmcp.Content{}
	}

	return sdk
}
