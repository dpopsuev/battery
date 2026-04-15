package mcpserver

import (
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// toSDKResult converts a Battery Result to an SDK CallToolResult.
// Since tool.Content IS sdkmcp.Content, no content translation is needed.
func toSDKResult(r tool.Result) *sdkmcp.CallToolResult {
	return r.ToSDK()
}
