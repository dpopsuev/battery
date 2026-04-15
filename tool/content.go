// Package tool re-exports MCP SDK content types directly.
// No translation layer — Battery uses the SDK's content model as-is.
package tool

import sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

// Content is the SDK's content interface. Used in Result.Content.
// Concrete types: *sdkmcp.TextContent, *sdkmcp.ImageContent,
// *sdkmcp.AudioContent, *sdkmcp.ResourceLink, *sdkmcp.EmbeddedResource.
type Content = sdkmcp.Content

// Re-export SDK content types so consumers import tool/ only.
type (
	TextContent      = sdkmcp.TextContent
	ImageContent     = sdkmcp.ImageContent
	AudioContent     = sdkmcp.AudioContent
	ResourceLink     = sdkmcp.ResourceLink
	EmbeddedResource = sdkmcp.EmbeddedResource
	ResourceContents = sdkmcp.ResourceContents
)
