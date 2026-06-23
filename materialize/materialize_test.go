package materialize_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/dpopsuev/battery/materialize"
	"github.com/dpopsuev/battery/translate"
)

const testSourceLocus = "locus" //nolint:goconst // test constant

var errConnRefused = errors.New("connection refused")
var errDiskFull = errors.New("disk full")

type stubSource struct {
	name    string
	records int
	err     error
	calls   int
}

func (s *stubSource) Name() string { return s.name }

func (s *stubSource) Pull(_ context.Context) (translate.Result, error) {
	s.calls++
	if s.err != nil {
		return translate.Result{}, s.err
	}
	records := make([]translate.Record, s.records)
	for i := range records {
		records[i] = translate.Record{ID: fmt.Sprintf("%s-%d", s.name, i), Kind: "test", Title: fmt.Sprintf("Record %d", i)}
	}
	return translate.Result{Records: records}, nil
}

type stubSink struct {
	pushed []sinkCall
	err    error
}

type sinkCall struct {
	source  string
	records int
	edges   int
}

func (s *stubSink) Push(_ context.Context, source string, result translate.Result) error {
	if s.err != nil {
		return s.err
	}
	s.pushed = append(s.pushed, sinkCall{source: source, records: len(result.Records), edges: len(result.Edges)})
	return nil
}

func TestMaterialize_AllSources(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	m.Register(&stubSource{name: testSourceLocus, records: 10}, 0)
	m.Register(&stubSource{name: "emcee", records: 5}, 0)

	results := m.Materialize(context.Background())
	if len(results) != 2 {
		t.Fatalf("want 2 results, got %d", len(results))
	}
	if results[0].Records != 10 || results[1].Records != 5 {
		t.Fatalf("unexpected record counts: %+v", results)
	}
	if len(sink.pushed) != 2 {
		t.Fatalf("want 2 pushes, got %d", len(sink.pushed))
	}
}

func TestMaterialize_SingleSource(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	m.Register(&stubSource{name: testSourceLocus, records: 10}, 0)
	m.Register(&stubSource{name: "emcee", records: 5}, 0)

	result, ok := m.MaterializeSource(context.Background(), "emcee")
	if !ok {
		t.Fatal("emcee source not found")
	}
	if result.Records != 5 {
		t.Fatalf("want 5 records, got %d", result.Records)
	}

	_, ok = m.MaterializeSource(context.Background(), "nonexistent")
	if ok {
		t.Fatal("expected not found for nonexistent source")
	}
}

func TestMaterialize_PullError(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	m.Register(&stubSource{name: "broken", err: errConnRefused}, 0)

	results := m.Materialize(context.Background())
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("expected error result, got %+v", results)
	}
	if len(sink.pushed) != 0 {
		t.Fatal("sink should not have been called on pull error")
	}
}

func TestMaterialize_PushError(t *testing.T) {
	sink := &stubSink{err: errDiskFull}
	m := materialize.New(sink)
	m.Register(&stubSource{name: testSourceLocus, records: 3}, 0)

	results := m.Materialize(context.Background())
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("expected error result, got %+v", results)
	}
}

func TestSweep_ExpiredTTL(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	src := &stubSource{name: testSourceLocus, records: 2}
	m.Register(src, 1*time.Millisecond)

	results := m.Sweep(context.Background())
	if len(results) != 1 {
		t.Fatalf("want 1 sweep result (never run = expired), got %d", len(results))
	}

	time.Sleep(2 * time.Millisecond)
	results = m.Sweep(context.Background())
	if len(results) != 1 {
		t.Fatalf("want 1 sweep result (TTL expired), got %d", len(results))
	}
	if src.calls != 2 {
		t.Fatalf("want 2 total pulls, got %d", src.calls)
	}
}

func TestSweep_FreshSkipped(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	m.Register(&stubSource{name: testSourceLocus, records: 2}, 1*time.Hour)

	m.Materialize(context.Background())

	results := m.Sweep(context.Background())
	if len(results) != 0 {
		t.Fatalf("want 0 sweep results (still fresh), got %d", len(results))
	}
}

func TestSweep_ZeroTTLSkipped(t *testing.T) {
	sink := &stubSink{}
	m := materialize.New(sink)
	m.Register(&stubSource{name: "manual", records: 1}, 0)

	results := m.Sweep(context.Background())
	if len(results) != 0 {
		t.Fatalf("want 0 sweep results (zero TTL = manual only), got %d", len(results))
	}
}

func TestSources(t *testing.T) {
	m := materialize.New(&stubSink{})
	m.Register(&stubSource{name: "a"}, 0)
	m.Register(&stubSource{name: "b"}, 0)

	names := m.Sources()
	if len(names) != 2 || names[0] != "a" || names[1] != "b" {
		t.Fatalf("want [a b], got %v", names)
	}
}
