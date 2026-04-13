package testkit

import (
	"context"
	"testing"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// StubMCPServer creates a minimal MCP server with the given tools and returns
// an in-memory transport for a client to connect to. Each tool returns "stub:<name>"
// as its result. The server session is cleaned up when t finishes.
func StubMCPServer(t *testing.T, tools map[string]string) sdkmcp.Transport {
	t.Helper()

	srv := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: "stub-mcp-server", Version: "v0.0.1"},
		nil,
	)

	for name, desc := range tools {
		toolName := name
		srv.AddTool(&sdkmcp.Tool{
			Name:        toolName,
			Description: desc,
			InputSchema: map[string]any{"type": "object"},
		}, func(_ context.Context, _ *sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
			return &sdkmcp.CallToolResult{
				Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: "stub:" + toolName}},
			}, nil
		})
	}

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()

	_, err := srv.Connect(context.Background(), serverTransport, nil)
	if err != nil {
		t.Fatalf("StubMCPServer: server connect: %v", err)
	}

	return clientTransport
}
