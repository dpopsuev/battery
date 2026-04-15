// Package mcpserver provides a Battery-integrated MCP server framework.
// It eliminates boilerplate by wrapping sdkmcp.Server with auto-Observable,
// result helpers, and a fluent builder API.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/dpopsuev/battery"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// ErrHandlerPanicked is returned when a tool handler panics.
var ErrHandlerPanicked = errors.New("battery: handler panicked")

// Log key constants.
const (
	logKeyTimeout = "timeout"
	logKeyPID     = "pid"
)

// DefaultInitTimeout is how long Serve waits for the MCP initialize handshake
// before exiting. Prevents silent hangs when the stdio pipe fails to connect.
const DefaultInitTimeout = 30 * time.Second

// Server wraps sdkmcp.Server with Battery conventions.
type Server struct {
	sdk          *sdkmcp.Server
	name         string
	version      string
	instructions string
	initTimeout  time.Duration
}

// NewServer creates a new Battery MCP server with the given name and version.
func NewServer(name, version string) *Server {
	return &Server{
		name:    name,
		version: version,
	}
}

// WithInstructions sets the MCP server instructions shown to clients.
func (s *Server) WithInstructions(instructions string) *Server {
	s.instructions = instructions
	return s
}

// build initializes the underlying sdkmcp.Server lazily on first use.
func (s *Server) build() {
	if s.sdk != nil {
		return
	}
	var opts *sdkmcp.ServerOptions
	if s.instructions != "" {
		opts = &sdkmcp.ServerOptions{Instructions: s.instructions}
	}
	version := s.version
	if battery.Version != "dev" {
		version = s.version + " (battery/" + battery.Version + ")"
	}
	s.sdk = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: s.name, Version: version},
		opts,
	)
}

// Tool registers a tool using server.ToolMeta for metadata and server.Handler
// for the handler function. The handler is auto-wrapped with Observable for
// timing/logging. InputSchema defaults to {"type":"object"}.
func (s *Server) Tool(meta server.ToolMeta, handler server.Handler) *Server {
	s.build()
	observed := server.Observable(meta.Name, handler)
	t := buildSDKTool(meta, map[string]any{"type": "object"}, nil)
	s.sdk.AddTool(t, adaptHandler(observed))
	return s
}

// ToolWithSchema registers a tool with an explicit JSON input schema.
func (s *Server) ToolWithSchema(meta server.ToolMeta, schema json.RawMessage, handler server.Handler) *Server {
	s.build()
	observed := server.Observable(meta.Name, handler)

	var schemaObj any
	if err := json.Unmarshal(schema, &schemaObj); err != nil {
		schemaObj = map[string]any{"type": "object"}
	}

	t := buildSDKTool(meta, schemaObj, nil)
	s.sdk.AddTool(t, adaptHandler(observed))
	return s
}

// ToolWithTool registers a battery tool.Tool directly, deriving annotations
// from optional interfaces (Cacheable, Gauged) and exposing ToolMeta in _meta.
func (s *Server) ToolWithTool(meta server.ToolMeta, bt tool.Tool) *Server {
	s.build()
	handler := func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
		return bt.Execute(ctx, input)
	}
	observed := server.Observable(meta.Name, handler)

	var schemaObj any
	if bt.InputSchema() != nil {
		if err := json.Unmarshal(bt.InputSchema(), &schemaObj); err != nil {
			schemaObj = map[string]any{"type": "object"}
		}
	} else {
		schemaObj = map[string]any{"type": "object"}
	}

	t := buildSDKTool(meta, schemaObj, bt)
	s.sdk.AddTool(t, adaptHandler(observed))
	return s
}

// buildSDKTool creates an sdkmcp.Tool from ToolMeta, deriving ToolAnnotations
// from optional interfaces and exposing metadata in _meta.
func buildSDKTool(meta server.ToolMeta, inputSchema any, bt tool.Tool) *sdkmcp.Tool {
	t := &sdkmcp.Tool{
		Name:        meta.Name,
		Description: meta.Description,
		InputSchema: inputSchema,
	}

	// Derive ToolAnnotations from optional interfaces.
	if bt != nil {
		t.Annotations = deriveAnnotations(bt)
	}

	// Expose ToolMeta in _meta for MCP client discovery.
	if len(meta.Keywords) > 0 || len(meta.Categories) > 0 || meta.Priority > 0 {
		t.Meta = sdkmcp.Meta{}
		if len(meta.Keywords) > 0 {
			t.Meta["battery.keywords"] = meta.Keywords
		}
		if len(meta.Categories) > 0 {
			t.Meta["battery.categories"] = meta.Categories
		}
		if meta.Priority > 0 {
			t.Meta["battery.priority"] = meta.Priority
		}
	}

	return t
}

// deriveAnnotations builds ToolAnnotations from a tool's optional interfaces.
func deriveAnnotations(bt tool.Tool) *sdkmcp.ToolAnnotations {
	ann := &sdkmcp.ToolAnnotations{}
	has := false

	if _, ok := bt.(tool.Cacheable); ok {
		ann.IdempotentHint = true
		has = true
	}

	if _, ok := bt.(tool.Gauged); ok {
		v := true
		ann.OpenWorldHint = &v
		has = true
	}

	if tm, ok := bt.(tool.ToolMetadata); ok {
		has = has || deriveDestructive(ann, tm.Metadata())
	}

	if !has {
		return nil
	}
	return ann
}

func deriveDestructive(ann *sdkmcp.ToolAnnotations, m tool.Metadata) bool {
	for _, cap := range m.Capabilities {
		if cap == "write" || cap == "delete" || cap == "destructive" {
			v := true
			ann.DestructiveHint = &v
			return true
		}
	}
	return false
}

// WithInitTimeout overrides the default 30s init handshake watchdog.
// Set to 0 to disable the watchdog entirely.
func (s *Server) WithInitTimeout(d time.Duration) *Server {
	s.initTimeout = d
	return s
}

// Serve starts the MCP server on the given transport. Blocks until ctx is canceled
// or the connection is closed.
//
// A watchdog goroutine exits the process if the MCP initialize handshake does not
// complete within initTimeout (default 30s). This prevents silent hangs when the
// stdio pipe fails to connect (see LCS-BUG-50).
func (s *Server) Serve(ctx context.Context, transport sdkmcp.Transport) error {
	s.build()

	timeout := s.initTimeout
	if timeout == 0 {
		timeout = DefaultInitTimeout
	}

	// Watchdog: exit if initialize handshake never arrives.
	// The SDK calls our first handler only after init completes,
	// so we detect init by checking for an active session.
	initDone := make(chan struct{})
	var closeOnce sync.Once
	cancelWatchdog := func() { closeOnce.Do(func() { close(initDone) }) }

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		deadline := time.After(timeout)
		for {
			select {
			case <-initDone:
				return
			case <-ctx.Done():
				return
			case <-deadline:
				slog.ErrorContext(ctx, "battery: MCP init watchdog fired — no initialize handshake received",
					slog.Duration(logKeyTimeout, timeout),
					slog.Int(logKeyPID, os.Getpid()),
				)
				os.Exit(1)
			case <-ticker.C:
				// Check if any session has been established.
				for range s.sdk.Sessions() {
					cancelWatchdog()
					return
				}
			}
		}
	}()

	err := s.sdk.Run(ctx, transport)
	cancelWatchdog()
	if err != nil {
		return fmt.Errorf("battery: server run: %w", err)
	}
	return nil
}

// SDK returns the underlying sdkmcp.Server for advanced use cases.
func (s *Server) SDK() *sdkmcp.Server {
	s.build()
	return s.sdk
}

// adaptHandler bridges server.Handler to sdkmcp.ToolHandler.
// server.Handler: func(ctx, json.RawMessage) (tool.Result, error)
// sdkmcp.ToolHandler: func(ctx, *CallToolRequest) (*CallToolResult, error)
//
// Includes panic recovery — a panicking handler returns error result, not a crash.
func adaptHandler(h server.Handler) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (res *sdkmcp.CallToolResult, retErr error) {
		defer func() {
			if r := recover(); r != nil {
				res = toSDKResult(tool.ErrorResult(fmt.Errorf("%w: %v", ErrHandlerPanicked, r)))
				retErr = nil
			}
		}()

		var input json.RawMessage
		if req.Params != nil {
			input = req.Params.Arguments
		}

		result, err := h(ctx, input)
		if err != nil {
			return toSDKResult(tool.ErrorResult(err)), nil
		}

		return toSDKResult(result), nil
	}
}
