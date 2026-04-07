package testkit_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dpopsuev/battery/testkit"
	"github.com/dpopsuev/battery/tool"
)

func TestStubTool_Execute(t *testing.T) {
	t.Parallel()
	s := testkit.NewStubTool("read", "Read a file")
	s.Result = "file contents"

	result, err := s.Execute(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "file contents" {
		t.Errorf("result = %q, want file contents", result)
	}
	if s.Calls != 1 {
		t.Errorf("Calls = %d, want 1", s.Calls)
	}
	if s.Name() != "read" {
		t.Errorf("Name = %q, want read", s.Name())
	}
}

func TestStubExecutor_Dispatch(t *testing.T) {
	t.Parallel()
	exec := testkit.NewStubExecutor(
		testkit.NewStubTool("read", "Read"),
		testkit.NewStubTool("write", "Write"),
	)

	result, err := exec.Execute(context.Background(), "read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if result != "ok" {
		t.Errorf("result = %q, want ok", result)
	}
	if len(exec.Calls) != 1 {
		t.Fatalf("Calls = %d, want 1", len(exec.Calls))
	}
	if exec.Calls[0].Name != "read" {
		t.Errorf("call name = %q, want read", exec.Calls[0].Name)
	}
}

func TestStubExecutor_NotFound(t *testing.T) {
	t.Parallel()
	exec := testkit.NewStubExecutor()

	_, err := exec.Execute(context.Background(), "missing", nil)
	if err != tool.ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestStubExecutor_Names(t *testing.T) {
	t.Parallel()
	exec := testkit.NewStubExecutor(
		testkit.NewStubTool("write", ""),
		testkit.NewStubTool("read", ""),
	)

	names := exec.Names()
	if len(names) != 2 || names[0] != "read" || names[1] != "write" {
		t.Errorf("Names() = %v, want [read write]", names)
	}
}

func TestStubGate_Allow(t *testing.T) {
	t.Parallel()
	g := &testkit.StubGate{Allow: true}

	v, err := g.Check(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Allowed {
		t.Error("expected Allowed = true")
	}
	if g.Calls != 1 {
		t.Errorf("Calls = %d, want 1", g.Calls)
	}
}

func TestStubGate_Deny(t *testing.T) {
	t.Parallel()
	g := &testkit.StubGate{Allow: false, Reason: "blocked"}

	v, _ := g.Check(context.Background(), "write", nil)
	if v.Allowed {
		t.Error("expected Allowed = false")
	}
	if v.Reason != "blocked" {
		t.Errorf("Reason = %q, want blocked", v.Reason)
	}
}

func TestStubSecurityGate(t *testing.T) {
	t.Parallel()
	sg := testkit.NewStubSecurityGate(true, "ok")

	v, err := sg.Check(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !v.Allowed {
		t.Error("expected Allowed = true")
	}
}

func TestStubEnricher(t *testing.T) {
	t.Parallel()
	e := &testkit.StubEnricher{Result: "symbols: 42 loaded"}

	result, err := e.Enrich(context.Background(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "symbols: 42 loaded" {
		t.Errorf("result = %q", result)
	}
	if e.Calls != 1 {
		t.Errorf("Calls = %d, want 1", e.Calls)
	}
}

func TestStubRecorder(t *testing.T) {
	t.Parallel()
	r := &testkit.StubRecorder{}

	r.Record(context.Background(), "read", nil, "output", nil, 0)
	if len(r.Records) != 1 {
		t.Fatalf("Records = %d, want 1", len(r.Records))
	}
	if r.Records[0].Tool != "read" {
		t.Errorf("tool = %q, want read", r.Records[0].Tool)
	}
}

func TestStubEnforcer(t *testing.T) {
	t.Parallel()
	e := &testkit.StubEnforcer{}

	err := e.Check(context.Background(), testkit.AllowAllToken(), "read", nil)
	if err != nil {
		t.Fatal(err)
	}
	if e.Calls != 1 {
		t.Errorf("Calls = %d, want 1", e.Calls)
	}
	if e.Last.Tool != "read" {
		t.Errorf("Last.Tool = %q, want read", e.Last.Tool)
	}
}
