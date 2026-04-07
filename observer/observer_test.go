package observer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRing_AppendAndLast(t *testing.T) {
	t.Parallel()
	r := NewRing(10)

	r.Append(Event{Action: "a"})
	r.Append(Event{Action: "b"})
	r.Append(Event{Action: "c"})

	last := r.Last(2)
	if len(last) != 2 || last[0].Action != "b" || last[1].Action != "c" {
		t.Errorf("Last(2) = %v", last)
	}
}

func TestRing_WrapAround(t *testing.T) {
	t.Parallel()
	r := NewRing(3)

	for i := range 5 {
		r.Append(Event{Detail: string(rune('a' + i))})
	}

	stats := r.Stats()
	if stats.Count != 3 {
		t.Errorf("Count = %d, want 3", stats.Count)
	}

	last := r.Last(3)
	if last[0].Detail != "c" || last[1].Detail != "d" || last[2].Detail != "e" {
		t.Errorf("expected [c d e], got %v", last)
	}
}

func TestRing_ByComponent(t *testing.T) {
	t.Parallel()
	r := NewRing(10)

	r.Append(Event{Component: ComponentTool, Action: "exec"})
	r.Append(Event{Component: ComponentAgent, Action: "think"})
	r.Append(Event{Component: ComponentTool, Action: "read"})

	tools := r.ByComponent(ComponentTool)
	if len(tools) != 2 {
		t.Errorf("ByComponent(tool) = %d, want 2", len(tools))
	}
}

func TestRing_Get(t *testing.T) {
	t.Parallel()
	r := NewRing(10)

	id := r.Append(Event{Action: "test"})
	e, ok := r.Get(id)
	if !ok || e.Action != "test" {
		t.Errorf("Get(%s) = %v, %v", id, e, ok)
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent")
	}
}

func TestTracer_BeginEnd(t *testing.T) {
	t.Parallel()
	r := NewRing(100)
	tracer := r.For(ComponentTool)

	rt := tracer.Begin("call", "read file")
	time.Sleep(1 * time.Millisecond)
	rt.End()

	events := r.Last(2)
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Action != "call" {
		t.Errorf("start action = %q, want call", events[0].Action)
	}
	if events[1].Action != "call_done" {
		t.Errorf("end action = %q, want call_done", events[1].Action)
	}
	if events[1].Latency <= 0 {
		t.Error("expected positive latency")
	}
	if events[1].ParentID != events[0].ID {
		t.Error("end event should reference start event as parent")
	}
}

func TestTracer_NilSafe(t *testing.T) {
	t.Parallel()
	var tracer *Tracer

	// Nil tracer should not panic.
	rt := tracer.Begin("call", "test")
	rt.End()
	rt.EndWithError()
	child := rt.Child("sub", "detail")
	child.End()
	tracer.Event("point", "test")
}

func TestRoundTrip_Child(t *testing.T) {
	t.Parallel()
	r := NewRing(100)
	tracer := r.For(ComponentAgent)

	parent := tracer.Begin("execute", "stage")
	child := parent.Child("tool_call", "read")
	child.End()
	parent.End()

	events := r.Last(4)
	// parent start, child start, child end, parent end
	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	// child's parent should be parent's ID
	if events[1].ParentID != events[0].ID {
		t.Error("child start should reference parent")
	}
}

func TestRoundTrip_WithServerTool(t *testing.T) {
	t.Parallel()
	r := NewRing(100)
	tracer := r.For(ComponentMCP)

	rt := tracer.Begin("call", "invoke").WithServer("scribe").WithTool("list")
	rt.End()

	done := r.Last(1)[0]
	if done.Server != "scribe" {
		t.Errorf("Server = %q, want scribe", done.Server)
	}
	if done.Tool != "list" {
		t.Errorf("Tool = %q, want list", done.Tool)
	}
}

func TestArchive_ExportImport(t *testing.T) {
	t.Parallel()
	r := NewRing(100)
	r.Append(Event{Component: ComponentTool, Action: "a"})
	r.Append(Event{Component: ComponentAgent, Action: "b"})
	r.Append(Event{Component: ComponentTool, Action: "c"})

	// Export only tool events.
	archive := Export(r, ComponentTool)
	if len(archive.Events) != 2 {
		t.Fatalf("exported %d events, want 2", len(archive.Events))
	}

	// Import into fresh ring.
	r2 := NewRing(100)
	Import(archive, r2)
	if r2.Stats().Count != 2 {
		t.Errorf("imported ring count = %d, want 2", r2.Stats().Count)
	}
}

func TestArchive_SaveLoadJSON(t *testing.T) {
	t.Parallel()
	r := NewRing(10)
	r.Append(Event{Component: ComponentTool, Action: "test", Detail: "detail"})

	archive := Export(r, "")
	path := filepath.Join(t.TempDir(), "archive.json")

	if err := archive.SaveJSON(path); err != nil {
		t.Fatal(err)
	}

	loaded, err := LoadArchive(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Events) != 1 {
		t.Fatalf("loaded %d events, want 1", len(loaded.Events))
	}
	if loaded.Events[0].Action != "test" {
		t.Errorf("action = %q, want test", loaded.Events[0].Action)
	}
}

func TestArchive_LoadNotFound(t *testing.T) {
	t.Parallel()
	_, err := LoadArchive("/nonexistent/path.json")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestDiff_DetectsNewErrors(t *testing.T) {
	t.Parallel()
	before := &Archive{Events: []Event{
		{Server: "s1", Tool: "t1", Action: "call_done", Latency: 10 * time.Millisecond},
	}}
	after := &Archive{Events: []Event{
		{Server: "s1", Tool: "t1", Action: "call_done", Latency: 10 * time.Millisecond},
		{Server: "s2", Tool: "t2", Action: "call_done", Error: true},
	}}

	result := Diff(before, after)
	if len(result.NewErrors) != 1 {
		t.Errorf("NewErrors = %d, want 1", len(result.NewErrors))
	}
	if result.EventCountBefore != 1 || result.EventCountAfter != 2 {
		t.Errorf("counts wrong: %d/%d", result.EventCountBefore, result.EventCountAfter)
	}
}

func TestRing_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	r := NewRing(100)
	done := make(chan bool, 2)

	go func() {
		for range 100 {
			r.Append(Event{Action: "write"})
		}
		done <- true
	}()
	go func() {
		for range 100 {
			r.Last(10)
		}
		done <- true
	}()

	<-done
	<-done

	if r.Stats().Count == 0 {
		t.Error("expected events after concurrent access")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
