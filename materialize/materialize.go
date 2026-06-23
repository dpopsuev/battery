// Package materialize provides the Source/Sink/Materializer pattern for
// bridging spoke data into a persistent artifact graph. Each spoke implements
// Source (pull domain data → translate.Result); the hub implements Sink
// (persist translate.Result). Materializer orchestrates pull→push with
// TTL-based freshness tracking.
package materialize

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/dpopsuev/battery/translate"
)

// Source pulls data from a spoke and returns canonical Records + Edges.
type Source interface {
	Name() string
	Pull(ctx context.Context) (translate.Result, error)
}

// Sink persists Records and Edges into a target store.
type Sink interface {
	Push(ctx context.Context, source string, result translate.Result) error
}

// Result summarizes a single source materialization.
type Result struct {
	Source  string `json:"source"`
	Records int    `json:"records"`
	Edges   int    `json:"edges"`
	Error   string `json:"error,omitempty"`
}

// Materializer orchestrates Sources → Sink with TTL-based freshness.
type Materializer struct {
	mu      sync.RWMutex
	sources []Source
	sink    Sink
	ttls    map[string]time.Duration
	lastRun map[string]time.Time
}

// New creates a Materializer with the given Sink.
func New(sink Sink) *Materializer {
	return &Materializer{
		sink:    sink,
		ttls:    make(map[string]time.Duration),
		lastRun: make(map[string]time.Time),
	}
}

// Register adds a Source with an optional TTL. Zero TTL means always fresh
// (only materialized on explicit Materialize call, never by Sweep).
func (m *Materializer) Register(src Source, ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sources = append(m.sources, src)
	if ttl > 0 {
		m.ttls[src.Name()] = ttl
	}
}

// Materialize pulls from all registered Sources and pushes to the Sink.
func (m *Materializer) Materialize(ctx context.Context) []Result {
	m.mu.RLock()
	sources := make([]Source, len(m.sources))
	copy(sources, m.sources)
	m.mu.RUnlock()

	results := make([]Result, 0, len(sources))
	for _, src := range sources {
		mr := m.materializeOne(ctx, src)
		results = append(results, mr)
	}
	return results
}

// MaterializeSource pulls from a single named Source and pushes to the Sink.
func (m *Materializer) MaterializeSource(ctx context.Context, name string) (Result, bool) {
	m.mu.RLock()
	var src Source
	for _, s := range m.sources {
		if s.Name() == name {
			src = s
			break
		}
	}
	m.mu.RUnlock()

	if src == nil {
		return Result{}, false
	}
	return m.materializeOne(ctx, src), true
}

// Sweep re-materializes any Source whose TTL has expired since its last run.
func (m *Materializer) Sweep(ctx context.Context) []Result {
	m.mu.RLock()
	now := time.Now()
	var stale []Source
	for _, src := range m.sources {
		ttl, ok := m.ttls[src.Name()]
		if !ok || ttl == 0 {
			continue
		}
		last, ran := m.lastRun[src.Name()]
		if !ran || now.Sub(last) > ttl {
			stale = append(stale, src)
		}
	}
	m.mu.RUnlock()

	results := make([]Result, 0, len(stale))
	for _, src := range stale {
		results = append(results, m.materializeOne(ctx, src))
	}
	return results
}

// Sources returns the names of all registered Sources.
func (m *Materializer) Sources() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	names := make([]string, len(m.sources))
	for i, s := range m.sources {
		names[i] = s.Name()
	}
	return names
}

func (m *Materializer) materializeOne(ctx context.Context, src Source) Result {
	result, err := src.Pull(ctx)
	if err != nil {
		slog.WarnContext(ctx, "materialize: pull failed",
			slog.String("source", src.Name()), slog.Any("error", err)) //nolint:sloglint // no constant
		return Result{Source: src.Name(), Error: err.Error()}
	}

	if err := m.sink.Push(ctx, src.Name(), result); err != nil {
		slog.WarnContext(ctx, "materialize: push failed",
			slog.String("source", src.Name()), slog.Any("error", err)) //nolint:sloglint // no constant
		return Result{Source: src.Name(), Records: len(result.Records), Error: err.Error()}
	}

	m.mu.Lock()
	m.lastRun[src.Name()] = time.Now()
	m.mu.Unlock()

	slog.InfoContext(ctx, "materialize: done",
		slog.String("source", src.Name()),
		slog.Int("records", len(result.Records)),
		slog.Int("edges", len(result.Edges))) //nolint:sloglint // no constant

	return Result{
		Source:  src.Name(),
		Records: len(result.Records),
		Edges:   len(result.Edges),
	}
}
