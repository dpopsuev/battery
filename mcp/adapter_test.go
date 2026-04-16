package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	battmcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

func TestMCPAdapter_RegisterDiscoverExecute(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Stand up a mock MCP server with 2 tools.
	clientTransport := testkit.StubMCPServer(t, map[string]string{
		"analysis": "Run code analysis",
		"lint":     "Run linter",
	})

	// Create adapter + registry.
	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)

	// Register MCP server.
	if err := adapter.RegisterMCP(ctx, "locus", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// Verify tools are registered with prefixed names.
	names := registry.Names()
	if len(names) != 2 {
		t.Fatalf("Names() = %v, want 2 tools", names)
	}
	if names[0] != "locus.analysis" || names[1] != "locus.lint" {
		t.Errorf("Names() = %v, want [locus.analysis locus.lint]", names)
	}

	// Verify tool metadata.
	tl, err := registry.Get("locus.analysis")
	if err != nil {
		t.Fatalf("Get(locus.analysis): %v", err)
	}
	if tl.Name() != "locus.analysis" {
		t.Errorf("Name() = %q", tl.Name())
	}
	if tl.Description() != "Run code analysis" {
		t.Errorf("Description() = %q", tl.Description())
	}

	// Execute a tool through the registry.
	result, err := registry.Execute(ctx, "locus.analysis", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Text() != "stub:analysis" {
		t.Errorf("result = %q, want %q", result.Text(), "stub:analysis")
	}

	// Unregister and verify tools are removed.
	if err := adapter.UnregisterMCP("locus"); err != nil {
		t.Fatalf("UnregisterMCP: %v", err)
	}
	if len(registry.Names()) != 0 {
		t.Errorf("after unregister: Names() = %v, want empty", registry.Names())
	}
}

// --- TSK-27: Middleware composability ---

func TestMCPAdapter_ClearanceFilters(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientTransport := testkit.StubMCPServer(t, map[string]string{
		"analysis": "Run code analysis",
		"lint":     "Run linter",
		"refactor": "Refactor code",
	})

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "locus", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// Clearance allows only 2 of 3 tools.
	cleared := tool.NewClearance(registry, []string{"locus.analysis", "locus.lint"})

	// Visible tools filtered.
	names := cleared.Names()
	if len(names) != 2 {
		t.Fatalf("cleared.Names() = %v, want 2 tools", names)
	}

	// Allowed tool executes.
	result, err := cleared.Execute(ctx, "locus.analysis", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Execute allowed: %v", err)
	}
	if result.Text() != "stub:analysis" {
		t.Errorf("result = %q", result.Text())
	}

	// Denied tool rejected.
	_, err = cleared.Execute(ctx, "locus.refactor", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error for denied tool")
	}
}

// --- TSK-28: Lifecycle ---

func TestMCPAdapter_Health(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientTransport := testkit.StubMCPServer(t, map[string]string{
		"ping": "Health check",
	})

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "svc", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	// Health on connected server succeeds.
	if err := adapter.Health(ctx, "svc"); err != nil {
		t.Errorf("Health: %v", err)
	}

	// Health on unknown server fails.
	if err := adapter.Health(ctx, "unknown"); err == nil {
		t.Error("expected error for unknown server")
	}
}

func TestMCPAdapter_Refresh(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientTransport := testkit.StubMCPServer(t, map[string]string{
		"analysis": "Run code analysis",
		"lint":     "Run linter",
	})

	registry := tool.NewRegistry()
	adapter := battmcp.NewMCPAdapter(registry)
	if err := adapter.RegisterMCP(ctx, "locus", clientTransport); err != nil {
		t.Fatalf("RegisterMCP: %v", err)
	}

	if len(registry.Names()) != 2 {
		t.Fatalf("before refresh: %d tools", len(registry.Names()))
	}

	// Refresh — server still has same 2 tools, no change.
	if err := adapter.Refresh(ctx, "locus"); err != nil {
		t.Fatalf("Refresh: %v", err)
	}

	if len(registry.Names()) != 2 {
		t.Errorf("after refresh: %d tools, want 2", len(registry.Names()))
	}

	// Refresh on unknown server fails.
	if err := adapter.Refresh(ctx, "unknown"); err == nil {
		t.Error("expected error for unknown server")
	}
}
