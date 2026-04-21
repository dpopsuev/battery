package mcp_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/battery/tool"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

// connectClient creates a paired in-memory transport, starts the server in
// background, and returns a connected client session.
func connectClient(t *testing.T, srv *battmcp.Server) *sdkmcp.ClientSession {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()

	go func() {
		_ = srv.Serve(ctx, serverTransport)
	}()

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// --- Test 1: Tool Registration ---

func TestServer_ToolRegistration(t *testing.T) {
	t.Parallel()

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "echo",
			Description: "Echo the input",
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.TextResult(string(input)), nil
		}).
		Tool(server.ToolMeta{
			Name:        "greet",
			Description: "Greet a person",
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var args struct {
				Name string `json:"name"`
			}
			json.Unmarshal(input, &args)
			return tool.TextResult("Hello, " + args.Name), nil
		})

	session := connectClient(t, srv)

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	if len(result.Tools) != 2 {
		t.Fatalf("listed %d tools, want 2", len(result.Tools))
	}

	names := make(map[string]bool)
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	if !names["echo"] || !names["greet"] {
		t.Errorf("tool names = %v, want echo and greet", names)
	}
}

// --- Test 1b: InputSchema from ToolMeta ---

func TestServer_ToolMetaInputSchema(t *testing.T) {
	t.Parallel()

	schema := json.RawMessage(`{
		"type": "object",
		"properties": {
			"action": {"type": "string", "enum": ["ping", "pong"]},
			"target": {"type": "string"}
		},
		"required": ["action"]
	}`)

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "with_schema",
			Description: "Tool with explicit schema",
			InputSchema: schema,
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.TextResult(string(input)), nil
		}).
		Tool(server.ToolMeta{
			Name:        "no_schema",
			Description: "Tool without schema",
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			return tool.TextResult(string(input)), nil
		})

	session := connectClient(t, srv)

	result, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	for _, tl := range result.Tools {
		schemaBytes, err := json.Marshal(tl.InputSchema)
		if err != nil {
			t.Fatalf("marshal schema for %s: %v", tl.Name, err)
		}
		var schemaMap map[string]any
		if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
			t.Fatalf("unmarshal schema for %s: %v", tl.Name, err)
		}

		switch tl.Name {
		case "with_schema":
			if _, ok := schemaMap["properties"]; !ok {
				t.Error("with_schema: expected properties in schema")
			}
			if _, ok := schemaMap["required"]; !ok {
				t.Error("with_schema: expected required in schema")
			}
		case "no_schema":
			if schemaMap["type"] != "object" {
				t.Errorf("no_schema: expected type=object, got %v", schemaMap["type"])
			}
			if _, ok := schemaMap["properties"]; ok {
				t.Error("no_schema: unexpected properties in bare schema")
			}
		}
	}
}

// --- Test 2: Tool Execution ---

func TestServer_ToolExecution(t *testing.T) {
	t.Parallel()

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "greet",
			Description: "Greet a person",
		}, func(_ context.Context, input json.RawMessage) (tool.Result, error) {
			var args struct {
				Name string `json:"name"`
			}
			json.Unmarshal(input, &args)
			return tool.TextResult("Hello, " + args.Name), nil
		})

	session := connectClient(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      "greet",
		Arguments: map[string]any{"name": "Battery"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatal("result.IsError = true")
	}
	if len(result.Content) == 0 {
		t.Fatal("no content in result")
	}

	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T, want *TextContent", result.Content[0])
	}
	if tc.Text != "Hello, Battery" {
		t.Errorf("result = %q, want %q", tc.Text, "Hello, Battery")
	}
}

// --- Test 3: Auto-Observable ---

type logRecord struct {
	Message string
	Attrs   map[string]any
}

type logBuffer struct {
	mu      sync.Mutex
	records []logRecord
}

func (b *logBuffer) hasMessage(msg string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, r := range b.records {
		if r.Message == msg {
			return true
		}
	}
	return false
}

type captureHandler struct {
	buf   *logBuffer
	attrs []slog.Attr
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

//nolint:gocritic // hugeParam: slog.Handler interface conformance
func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := logRecord{
		Message: r.Message,
		Attrs:   make(map[string]any),
	}
	for _, a := range h.attrs {
		rec.Attrs[a.Key] = a.Value.Any()
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value.Any()
		return true
	})
	h.buf.mu.Lock()
	h.buf.records = append(h.buf.records, rec)
	h.buf.mu.Unlock()
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newAttrs := make([]slog.Attr, len(h.attrs)+len(attrs))
	copy(newAttrs, h.attrs)
	copy(newAttrs[len(h.attrs):], attrs)
	return &captureHandler{buf: h.buf, attrs: newAttrs}
}

func (h *captureHandler) WithGroup(_ string) slog.Handler { return h }

func TestServer_AutoObservable(t *testing.T) {
	t.Parallel()

	buf := &logBuffer{}
	logger := slog.New(&captureHandler{buf: buf})
	old := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(old)

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "ping",
			Description: "Ping",
		}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult("pong"), nil
		})

	session := connectClient(t, srv)

	_, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "ping",
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	// Observable should have logged "battery: tool completed".
	if !buf.hasMessage("battery: tool completed") {
		t.Error("expected 'battery: tool completed' log from Observable wrapper")
	}
}

// --- Test 4: Panic Recovery ---

func TestServer_PanicRecovery(t *testing.T) {
	t.Parallel()

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "boom",
			Description: "Panics on every call",
		}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			panic("kaboom")
		})

	session := connectClient(t, srv)

	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name: "boom",
	})
	if err != nil {
		t.Fatalf("CallTool should not return protocol error: %v", err)
	}
	if !result.IsError {
		t.Fatal("expected IsError=true for panicking handler")
	}

	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("content type = %T", result.Content[0])
	}
	if tc.Text == "" {
		t.Error("error content should not be empty")
	}
}

// --- Test 5: Context Cancellation ---

func TestServer_ContextCancellation(t *testing.T) {
	t.Parallel()

	handlerCalled := make(chan struct{})
	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "slow",
			Description: "Blocks until context canceled",
		}, func(ctx context.Context, _ json.RawMessage) (tool.Result, error) {
			close(handlerCalled)
			<-ctx.Done()
			return tool.Result{}, ctx.Err()
		})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer session.Close()

	// Call in background, cancel after handler confirms it's running.
	callCtx, callCancel := context.WithCancel(ctx)
	errCh := make(chan error, 1)
	go func() {
		_, err := session.CallTool(callCtx, &sdkmcp.CallToolParams{Name: "slow"})
		errCh <- err
	}()

	<-handlerCalled
	callCancel()

	err = <-errCh
	if err == nil {
		t.Fatal("expected error after context cancellation")
	}
}

// --- Test 6: Graceful Shutdown ---

func TestServer_GracefulShutdown(t *testing.T) {
	t.Parallel()

	srv := battmcp.NewServer("test-server", "v0.1.0").
		Tool(server.ToolMeta{
			Name:        "ping",
			Description: "Ping",
		}, func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.TextResult("pong"), nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()

	serveDone := make(chan error, 1)
	go func() {
		serveDone <- srv.Serve(ctx, serverTransport)
	}()

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	// Tool works before shutdown.
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{Name: "ping"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	tc, _ := result.Content[0].(*sdkmcp.TextContent)
	if tc.Text != "pong" {
		t.Errorf("result = %q", tc.Text)
	}

	// Cancel context — server should shut down.
	cancel()
	session.Close()

	select {
	case err := <-serveDone:
		// Serve returned — success. Error may be nil or context.Canceled.
		_ = err
	case <-time.After(5 * time.Second):
		t.Fatal("Serve did not return after context cancellation")
	}
}
