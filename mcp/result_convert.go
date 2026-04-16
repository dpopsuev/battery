package mcp

import (
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// batteryToSDKConverter converts a Battery Content to an SDK Content.
type batteryToSDKConverter func(tool.Content) (sdkmcp.Content, bool)

// toSDKConverters maps Battery content types to SDK converters.
var toSDKConverters []batteryToSDKConverter //nolint:gochecknoglobals // strategy registry

func init() {
	toSDKConverters = []batteryToSDKConverter{
		func(c tool.Content) (sdkmcp.Content, bool) {
			if v, ok := c.(tool.TextContent); ok {
				return &sdkmcp.TextContent{Text: v.Text}, true
			}
			return nil, false
		},
		func(c tool.Content) (sdkmcp.Content, bool) {
			if v, ok := c.(tool.ImageContent); ok {
				return &sdkmcp.ImageContent{MIMEType: v.MIMEType, Data: v.Data}, true
			}
			return nil, false
		},
		func(c tool.Content) (sdkmcp.Content, bool) {
			if v, ok := c.(tool.AudioContent); ok {
				return &sdkmcp.AudioContent{MIMEType: v.MIMEType, Data: v.Data}, true
			}
			return nil, false
		},
		func(c tool.Content) (sdkmcp.Content, bool) {
			if v, ok := c.(tool.ResourceLink); ok {
				return &sdkmcp.ResourceLink{URI: v.URI, Name: v.Name, Description: v.Description, MIMEType: v.MIMEType}, true
			}
			return nil, false
		},
		func(c tool.Content) (sdkmcp.Content, bool) {
			if v, ok := c.(tool.ResourceContent); ok {
				return &sdkmcp.EmbeddedResource{
					Resource: &sdkmcp.ResourceContents{URI: v.URI, MIMEType: v.MIMEType, Text: v.Text, Blob: v.Blob},
				}, true
			}
			return nil, false
		},
	}
}

// resultToSDK translates a Battery Result to an SDK CallToolResult.
func resultToSDK(r tool.Result) *sdkmcp.CallToolResult {
	sdk := &sdkmcp.CallToolResult{
		IsError:           r.IsError,
		StructuredContent: r.StructuredContent,
	}
	for _, bc := range r.Content {
		for _, conv := range toSDKConverters {
			if sc, ok := conv(bc); ok {
				sdk.Content = append(sdk.Content, sc)
				break
			}
		}
	}
	if sdk.Content == nil {
		sdk.Content = []sdkmcp.Content{}
	}
	return sdk
}
