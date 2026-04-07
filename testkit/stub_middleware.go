package testkit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/dpopsuev/battery/middleware"
)

// StubGate implements middleware.Gate. Configurable allow/deny.
type StubGate struct {
	Allow    bool
	Reason   string
	CheckErr error

	mu    sync.Mutex
	Calls int
}

var _ middleware.Gate = (*StubGate)(nil)

func (g *StubGate) Check(_ context.Context, _ string, _ json.RawMessage) (middleware.Verdict, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Calls++
	if g.CheckErr != nil {
		return middleware.Verdict{}, g.CheckErr
	}
	return middleware.Verdict{Allowed: g.Allow, Reason: g.Reason}, nil
}

// NewStubSecurityGate wraps a StubGate as a middleware.SecurityGate.
func NewStubSecurityGate(allow bool, reason string) middleware.SecurityGate {
	return middleware.AsSecurityGate(&StubGate{Allow: allow, Reason: reason})
}

// StubEnricher implements middleware.Enricher.
type StubEnricher struct {
	Result string
	Err    error

	mu    sync.Mutex
	Calls int
}

var _ middleware.Enricher = (*StubEnricher)(nil)

func (e *StubEnricher) Enrich(_ context.Context, _ string, _ json.RawMessage) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Calls++
	return e.Result, e.Err
}

// StubRecorder implements middleware.Recorder.
type StubRecorder struct {
	mu      sync.Mutex
	Records []StubRecordEntry
}

// StubRecordEntry captures one Record call.
type StubRecordEntry struct {
	Tool    string
	Input   json.RawMessage
	Output  string
	Err     error
	Elapsed time.Duration
}

var _ middleware.Recorder = (*StubRecorder)(nil)

func (r *StubRecorder) Record(_ context.Context, tool string, input json.RawMessage, output string, err error, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Records = append(r.Records, StubRecordEntry{
		Tool: tool, Input: input, Output: output, Err: err, Elapsed: elapsed,
	})
}
