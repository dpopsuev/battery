package workbench_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
	"github.com/dpopsuev/battery/workbench"
)

func TestWorkbench_Craft(t *testing.T) {
	t.Parallel()
	w := workbench.New()

	stub := testkit.NewStubTool("read", "Read a file")
	stub.Result = "file contents"
	w.Craft(stub)

	result, err := w.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "file contents" {
		t.Errorf("result = %q", result.Text())
	}
}

func TestWorkbench_Mount(t *testing.T) {
	t.Parallel()
	reg := tool.NewRegistry()
	stub := testkit.NewStubTool("analyze", "")
	stub.Result = "analysis"
	reg.Register(stub)

	w := workbench.New().Mount(reg)

	result, err := w.Execute(context.Background(), "analyze", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "analysis" {
		t.Errorf("result = %q", result.Text())
	}
}

func TestWorkbench_NotFound(t *testing.T) {
	t.Parallel()
	w := workbench.New()

	_, err := w.Execute(context.Background(), "missing", nil)
	if !errors.Is(err, tool.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkbench_Names(t *testing.T) {
	t.Parallel()
	w := workbench.New()
	w.Craft(testkit.NewStubTool("beta", ""))
	w.Craft(testkit.NewStubTool("alpha", ""))

	names := w.Names()
	if len(names) != 2 || names[0] != "alpha" || names[1] != "beta" {
		t.Errorf("Names() = %v, want [alpha beta]", names)
	}
}

func TestWorkbench_Pipe(t *testing.T) {
	t.Parallel()

	upper := testkit.NewStubTool("upper", "to uppercase")
	upper.Result = "HELLO"

	prefix := testkit.NewStubTool("prefix", "add prefix")
	prefix.Result = "PREFIX:HELLO"

	w := workbench.New().
		Craft(upper).
		Craft(prefix).
		Pipe("upper-then-prefix", "upper", "prefix")

	result, err := w.Execute(context.Background(), "upper-then-prefix", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatal(err)
	}
	// The final output is the last step's result.
	if result.Text() != "PREFIX:HELLO" {
		t.Errorf("pipe result = %q, want PREFIX:HELLO", result.Text())
	}
}

func TestWorkbench_Pipe_StepNotFound(t *testing.T) {
	t.Parallel()
	w := workbench.New().Pipe("broken", "missing-step")

	_, err := w.Execute(context.Background(), "broken", nil)
	if err == nil {
		t.Error("expected error for missing pipe step")
	}
}

func TestWorkbench_Swap_Primary(t *testing.T) {
	t.Parallel()
	primary := testkit.NewStubTool("read", "fast read")
	primary.Result = "primary-result"
	fallback := testkit.NewStubTool("read", "slow read")
	fallback.Result = "fallback-result"

	w := workbench.New().Swap(workbench.SwapRule{
		Name:      "read",
		Predicate: func() bool { return true },
		Primary:   primary,
		Fallback:  fallback,
	})

	result, err := w.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "primary-result" {
		t.Errorf("result = %q, want primary-result", result.Text())
	}
}

func TestWorkbench_Swap_Fallback(t *testing.T) {
	t.Parallel()
	primary := testkit.NewStubTool("read", "fast read")
	primary.Result = "primary-result"
	fallback := testkit.NewStubTool("read", "slow read")
	fallback.Result = "fallback-result"

	w := workbench.New().Swap(workbench.SwapRule{
		Name:      "read",
		Predicate: func() bool { return false },
		Primary:   primary,
		Fallback:  fallback,
	})

	result, err := w.Execute(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Text() != "fallback-result" {
		t.Errorf("result = %q, want fallback-result", result.Text())
	}
}

func TestWorkbench_NamesIncludesPipes(t *testing.T) {
	t.Parallel()
	w := workbench.New().
		Craft(testkit.NewStubTool("step1", "")).
		Pipe("my-pipe", "step1")

	names := w.Names()
	hasPipe := false
	for _, n := range names {
		if n == "my-pipe" {
			hasPipe = true
		}
	}
	if !hasPipe {
		t.Errorf("Names() = %v, missing pipe name my-pipe", names)
	}
}

func TestWorkbench_All(t *testing.T) {
	t.Parallel()
	w := workbench.New().
		Craft(testkit.NewStubTool("a", "")).
		Craft(testkit.NewStubTool("b", ""))

	all := w.All()
	if len(all) != 2 {
		t.Errorf("All() = %d, want 2", len(all))
	}
}
