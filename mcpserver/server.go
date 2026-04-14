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

	"github.com/dpopsuev/battery/server"
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
	s.sdk = sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: s.name, Version: s.version},
		opts,
	)
}

// Tool registers a tool using server.ToolMeta for metadata and server.Handler
// for the handler function. The handler is auto-wrapped with Observable for
// timing/logging. InputSchema defaults to {"type":"object"}.
func (s *Server) Tool(meta server.ToolMeta, handler server.Handler) *Server {
	s.build()

	observed := server.Observable(meta.Name, handler)

	s.sdk.AddTool(
		&sdkmcp.Tool{
			Name:        meta.Name,
			Description: meta.Description,
			InputSchema: map[string]any{"type": "object"},
		},
		adaptHandler(observed),
	)
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

	s.sdk.AddTool(
		&sdkmcp.Tool{
			Name:        meta.Name,
			Description: meta.Description,
			InputSchema: schemaObj,
		},
		adaptHandler(observed),
	)
	return s
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
// server.Handler: func(ctx, json.RawMessage) (string, error)
// sdkmcp.ToolHandler: func(ctx, *CallToolRequest) (*CallToolResult, error)
//
// Includes panic recovery — a panicking handler returns ErrorResult, not a crash.
func adaptHandler(h server.Handler) sdkmcp.ToolHandler {
	return func(ctx context.Context, req *sdkmcp.CallToolRequest) (res *sdkmcp.CallToolResult, retErr error) {
		defer func() {
			if r := recover(); r != nil {
				res = ErrorResult(fmt.Errorf("%w: %v", ErrHandlerPanicked, r))
				retErr = nil
			}
		}()

		var input json.RawMessage
		if req.Params != nil {
			input = req.Params.Arguments
		}

		result, err := h(ctx, input)
		if err != nil {
			return ErrorResult(err), nil
		}

		return TextResult(result), nil
	}
}
