// Package mcp bridges external MCP servers into Battery's tool.Executor world.
// Given an MCP server connection, it calls tools/list to discover the catalog
// and auto-registers each as a battery/tool.Tool backed by tools/call.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrServerNotFound is returned when the named MCP server is not registered.
var ErrServerNotFound = errors.New("mcp server not found")

// ErrMCPToolError is returned when an MCP tool call returns IsError=true.
var ErrMCPToolError = errors.New("mcp tool error")

// serverConn holds the client session and metadata for one connected MCP server.
type serverConn struct {
	name    string
	client  *sdkmcp.Client
	session *sdkmcp.ClientSession
	tools   []string // tool names registered in the registry (prefixed)
}

// MCPAdapter manages connections to external MCP servers and registers
// their tools as battery/tool.Tool instances in a tool.Registry.
type MCPAdapter struct {
	mu       sync.Mutex
	registry *tool.Registry
	servers  map[string]*serverConn
}

// NewMCPAdapter creates an adapter that registers MCP-backed tools into the given registry.
func NewMCPAdapter(registry *tool.Registry) *MCPAdapter {
	return &MCPAdapter{
		registry: registry,
		servers:  make(map[string]*serverConn),
	}
}

// RegisterMCP connects to the MCP server over the given transport, calls tools/list,
// and registers each discovered tool into the registry as a tool.Tool.
// The name parameter prefixes tool names to avoid collisions: "servername.toolname".
func (a *MCPAdapter) RegisterMCP(ctx context.Context, name string, transport sdkmcp.Transport) error {
	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "battery-mcp-adapter", Version: "v0.1.0"},
		nil,
	)

	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("mcp connect %q: %w", name, err)
	}

	listResult, err := session.ListTools(ctx, nil)
	if err != nil {
		session.Close()
		return fmt.Errorf("mcp list tools %q: %w", name, err)
	}

	conn := &serverConn{
		name:    name,
		client:  client,
		session: session,
		tools:   make([]string, 0, len(listResult.Tools)),
	}

	for _, t := range listResult.Tools {
		var schema json.RawMessage
		if t.InputSchema != nil {
			data, err := json.Marshal(t.InputSchema)
			if err == nil {
				schema = data
			}
		}

		mt := &mcpTool{
			serverName:  name,
			name:        t.Name,
			description: t.Description,
			schema:      schema,
			session:     session,
		}

		prefixed := mt.Name()
		a.registry.Register(mt)
		conn.tools = append(conn.tools, prefixed)
	}

	a.mu.Lock()
	a.servers[name] = conn
	a.mu.Unlock()

	return nil
}

// UnregisterMCP disconnects from the named MCP server and removes its tools from the registry.
func (a *MCPAdapter) UnregisterMCP(name string) error {
	a.mu.Lock()
	conn, ok := a.servers[name]
	if !ok {
		a.mu.Unlock()
		return fmt.Errorf("%w: %q", ErrServerNotFound, name)
	}
	delete(a.servers, name)
	a.mu.Unlock()

	for _, toolName := range conn.tools {
		a.registry.Unregister(toolName)
	}

	return conn.session.Close()
}

// Refresh re-queries tools/list for the named server and updates the registry.
// New tools are registered; removed tools are deregistered.
func (a *MCPAdapter) Refresh(ctx context.Context, name string) error {
	a.mu.Lock()
	conn, ok := a.servers[name]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrServerNotFound, name)
	}

	listResult, err := conn.session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("mcp refresh %q: %w", name, err)
	}

	// Build new tool set.
	newTools := make(map[string]bool, len(listResult.Tools))
	for _, t := range listResult.Tools {
		prefixed := name + "." + t.Name
		newTools[prefixed] = true

		// Register if not already present.
		if _, err := a.registry.Get(prefixed); err != nil {
			var schema json.RawMessage
			if t.InputSchema != nil {
				if data, err := json.Marshal(t.InputSchema); err == nil {
					schema = data
				}
			}
			a.registry.Register(&mcpTool{
				serverName:  name,
				name:        t.Name,
				description: t.Description,
				schema:      schema,
				session:     conn.session,
			})
		}
	}

	// Remove tools that no longer exist on the server.
	a.mu.Lock()
	var remaining []string
	for _, toolName := range conn.tools {
		if newTools[toolName] {
			remaining = append(remaining, toolName)
		} else {
			a.registry.Unregister(toolName)
		}
	}
	// Add any newly discovered tools.
	for _, t := range listResult.Tools {
		prefixed := name + "." + t.Name
		found := false
		for _, existing := range conn.tools {
			if existing == prefixed {
				found = true
				break
			}
		}
		if !found {
			remaining = append(remaining, prefixed)
		}
	}
	conn.tools = remaining
	a.mu.Unlock()

	return nil
}

// Health checks the connection to the named MCP server by calling tools/list.
func (a *MCPAdapter) Health(ctx context.Context, name string) error {
	a.mu.Lock()
	conn, ok := a.servers[name]
	a.mu.Unlock()
	if !ok {
		return fmt.Errorf("%w: %q", ErrServerNotFound, name)
	}

	_, err := conn.session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("mcp health %q: %w", name, err)
	}
	return nil
}
