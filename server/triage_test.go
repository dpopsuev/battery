package server_test

import (
	"testing"

	"github.com/dpopsuev/battery/server"
)

func newTestTriageRegistry() *server.TriageRegistry {
	r := server.NewTriageRegistry()
	r.Register(server.ToolMeta{
		Name:        "get_hot_spots",
		Description: "High fan-in + high churn components",
		Keywords:    []string{"perf", "bottleneck", "hot", "slow", "risk", "churn"},
		Categories:  []string{"performance", "refactoring"},
		DefaultArgs: map[string]any{"top_n": 10},
		Rationale:   map[string]string{"performance": "High fan-in + high churn = likely bottleneck"},
		Priority:    1,
	})
	r.Register(server.ToolMeta{
		Name:        "scan_project",
		Description: "Full architecture scan",
		Keywords:    []string{"architecture", "scan", "structure", "overview"},
		Categories:  []string{"architecture", "general"},
		Priority:    0,
	})
	r.Register(server.ToolMeta{
		Name:        "get_dependencies",
		Description: "Fan-in and fan-out edges",
		Keywords:    []string{"dependency", "depends", "import", "coupling"},
		Categories:  []string{"architecture", "refactoring"},
		Priority:    2,
	})
	return r
}

func TestTriage_PerformanceIntent(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("find performance bottlenecks", "")
	if result.Category != "performance" {
		t.Errorf("category = %q, want performance", result.Category)
	}
	if len(result.Tools) == 0 {
		t.Fatal("expected at least one tool")
	}
	if result.Tools[0].Name != "get_hot_spots" {
		t.Errorf("top tool = %q, want get_hot_spots", result.Tools[0].Name)
	}
	if result.Confidence <= 0 {
		t.Errorf("confidence = %f, want > 0", result.Confidence)
	}
}

func TestTriage_ArchitectureIntent(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("show me the architecture overview", "")
	if result.Category != "architecture" {
		t.Errorf("category = %q, want architecture", result.Category)
	}
	if len(result.Tools) < 2 {
		t.Fatalf("expected at least 2 tools, got %d", len(result.Tools))
	}
	// scan_project has priority 0, should be first.
	if result.Tools[0].Name != "scan_project" {
		t.Errorf("top tool = %q, want scan_project", result.Tools[0].Name)
	}
}

func TestTriage_RationaleInReason(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("what are the performance hot spots", "")
	for _, m := range result.Tools {
		if m.Name == "get_hot_spots" {
			if m.Reason != "High fan-in + high churn = likely bottleneck" {
				t.Errorf("reason = %q, want rationale text", m.Reason)
			}
			return
		}
	}
	t.Error("get_hot_spots not in results")
}

func TestTriage_PathInjection(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("scan architecture", "/my/repo")
	for _, m := range result.Tools {
		if p, ok := m.Params["path"]; ok {
			if p != "/my/repo" {
				t.Errorf("path = %v, want /my/repo", p)
			}
			return
		}
	}
	t.Error("path not injected into params")
}

func TestTriage_DefaultArgs(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("performance bottleneck hot spots", "")
	for _, m := range result.Tools {
		if m.Name == "get_hot_spots" {
			if v, ok := m.Params["top_n"]; !ok || v != 10 {
				t.Errorf("top_n = %v, want 10", v)
			}
			return
		}
	}
	t.Error("get_hot_spots not in results")
}

func TestTriage_EmptyIntent(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	result := r.Triage("", "")
	if result.Category != "general" {
		t.Errorf("category = %q, want general", result.Category)
	}
}

func TestTriage_EmptyRegistry(t *testing.T) {
	t.Parallel()
	r := server.NewTriageRegistry()

	result := r.Triage("find bugs", "")
	if result.Category != "general" {
		t.Errorf("category = %q, want general", result.Category)
	}
	if len(result.Tools) != 0 {
		t.Errorf("tools = %d, want 0", len(result.Tools))
	}
}

func TestTriageRegistry_List(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	all := r.List()
	if len(all) != 3 {
		t.Errorf("list = %d, want 3", len(all))
	}
}

func TestTriageRegistry_ByCategory(t *testing.T) {
	t.Parallel()
	r := newTestTriageRegistry()

	arch := r.ByCategory("architecture")
	if len(arch) != 2 {
		t.Errorf("architecture tools = %d, want 2", len(arch))
	}

	perf := r.ByCategory("performance")
	if len(perf) != 1 {
		t.Errorf("performance tools = %d, want 1", len(perf))
	}
}
