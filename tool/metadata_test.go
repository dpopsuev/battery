package tool_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/tool"
)

// CapabilityDeclarerContract validates any CapabilityDeclarer implementation.
func CapabilityDeclarerContract(t *testing.T, newDeclarer func(caps []string) tool.CapabilityDeclarer) {
	t.Helper()

	t.Run("ReturnsDeclaredCapabilities", func(t *testing.T) {
		caps := []string{"filesystem:read", "network:http"}
		cd := newDeclarer(caps)
		got := cd.RequiredCapabilities()
		if len(got) != len(caps) {
			t.Fatalf("got %d capabilities, want %d", len(got), len(caps))
		}
		for i, c := range got {
			if c != caps[i] {
				t.Errorf("capability[%d] = %q, want %q", i, c, caps[i])
			}
		}
	})

	t.Run("EmptyCapabilities", func(t *testing.T) {
		cd := newDeclarer(nil)
		got := cd.RequiredCapabilities()
		if len(got) != 0 {
			t.Errorf("expected empty capabilities, got %v", got)
		}
	})
}

// Verify CapabilityDeclarer is an optional interface on Tool (type assertion check).
func TestCapabilityDeclarer_OptionalOnTool(t *testing.T) {
	// A plain Tool that does NOT implement CapabilityDeclarer.
	var plainTool tool.Tool = stubPlainTool{}
	if _, ok := plainTool.(tool.CapabilityDeclarer); ok {
		t.Error("plain Tool should not implement CapabilityDeclarer")
	}
}

// stubPlainTool is a minimal Tool that does not implement CapabilityDeclarer.
type stubPlainTool struct{}

func (stubPlainTool) Name() string                 { return "plain" }
func (stubPlainTool) Description() string          { return "" }
func (stubPlainTool) InputSchema() json.RawMessage { return nil }

func (stubPlainTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return tool.Result{}, nil
}
