package tool_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

// ExecutorContract runs the Executor contract suite against any implementation.
// Every Executor (Registry, Envelope, Clearance, StubExecutor) must pass.
func ExecutorContract(t *testing.T, newExecutor func(tools ...tool.Tool) tool.Executor) {
	t.Helper()

	t.Run("ExecuteRegistered", func(t *testing.T) {
		t.Parallel()
		stub := testkit.NewStubTool("read", "Read a file")
		stub.Result = "file contents"
		exec := newExecutor(stub)

		result, err := exec.Execute(context.Background(), "read", json.RawMessage(`{}`))
		if err != nil {
			t.Fatal(err)
		}
		if result != "file contents" {
			t.Errorf("result = %q, want file contents", result)
		}
	})

	t.Run("ExecuteUnknown", func(t *testing.T) {
		t.Parallel()
		exec := newExecutor(testkit.NewStubTool("read", ""))

		_, err := exec.Execute(context.Background(), "nonexistent", nil)
		if !errors.Is(err, tool.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got %v", err)
		}
	})

	t.Run("AllReturnsRegistered", func(t *testing.T) {
		t.Parallel()
		exec := newExecutor(
			testkit.NewStubTool("read", "Read"),
			testkit.NewStubTool("write", "Write"),
		)

		all := exec.All()
		if len(all) != 2 {
			t.Fatalf("All() = %d tools, want 2", len(all))
		}
	})

	t.Run("NamesSorted", func(t *testing.T) {
		t.Parallel()
		exec := newExecutor(
			testkit.NewStubTool("write", ""),
			testkit.NewStubTool("read", ""),
			testkit.NewStubTool("bash", ""),
		)

		names := exec.Names()
		if len(names) != 3 {
			t.Fatalf("Names() = %d, want 3", len(names))
		}
		for i := 1; i < len(names); i++ {
			if names[i] < names[i-1] {
				t.Errorf("Names() not sorted: %v", names)
				break
			}
		}
	})
}

// TestStubExecutor_Contract proves the stub passes the contract.
func TestStubExecutor_Contract(t *testing.T) {
	ExecutorContract(t, func(tools ...tool.Tool) tool.Executor {
		return testkit.NewStubExecutor(tools...)
	})
}

// TestRegistry_Contract proves the real Registry passes the same contract.
func TestRegistry_Contract(t *testing.T) {
	ExecutorContract(t, func(tools ...tool.Tool) tool.Executor {
		r := tool.NewRegistry()
		for _, tt := range tools {
			r.Register(tt)
		}
		return r
	})
}
