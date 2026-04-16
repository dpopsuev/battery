package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// ErrNoSecurityGate indicates Build() was called without a SecurityGate.
var ErrNoSecurityGate = errors.New("battery: cannot build Envelope without SecurityGate — security by construction")

// Envelope wraps a tool.Executor with Gate → Enrich → Execute → Record pipeline.
// Implements tool.Executor (LSP — substitutable for any Executor).
type Envelope struct {
	gates            []Gate
	enrichers        []Enricher
	recorders        []Recorder
	executor         tool.Executor
	maxResultDefault int        // default max output bytes; 0 = unlimited
	gaugeFunc        GaugeFunc  // optional; nil = no gauge collection
	cacheStore       CacheStore // optional; nil = no caching
	cacheHook        CacheHook  // optional; nil = no cache event notifications
	cacheTTL         time.Duration
}

var _ tool.Executor = (*Envelope)(nil)

// Execute runs the full pipeline: check gates → enrich → execute → record.
func (e *Envelope) Execute(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
	// Gates.
	for _, g := range e.gates {
		v, err := g.Check(ctx, name, input)
		if err != nil {
			return tool.Result{}, fmt.Errorf("%w: gate error: %w", ErrToolDenied, err)
		}
		if !v.Allowed {
			return tool.Result{}, fmt.Errorf("%w: %s", ErrToolDenied, v.Reason)
		}
	}

	// Enrichers (non-fatal, append as additional TextContent blocks).
	var enrichments []tool.Content
	for _, en := range e.enrichers {
		result, err := en.Enrich(ctx, name, input)
		if err == nil && result != "" {
			enrichments = append(enrichments, tool.TextContent{Text: result})
		}
	}

	// Cache check (after gates, before execute).
	cacheKey, cached := e.checkCache(ctx, name, input)
	cacheHit := cached != nil

	var output tool.Result
	var execErr error
	var elapsed time.Duration

	if cacheHit {
		output = *cached
	} else {
		// Execute.
		start := time.Now()
		output, execErr = e.executor.Execute(ctx, name, input)
		elapsed = time.Since(start)

		// Cache store (untruncated result).
		if execErr == nil && cacheKey != "" {
			e.storeCache(ctx, name, cacheKey, output)
		}
	}

	// Append enrichments to result on success.
	if execErr == nil && len(enrichments) > 0 {
		output.Content = append(output.Content, enrichments...)
	}

	// Truncate text output if MaxResultSize is configured.
	if execErr == nil {
		output = e.truncateResult(name, output)
	}

	// Gauge (optional, non-blocking, skip on cache hit — tool didn't run).
	if execErr == nil && e.gaugeFunc != nil && !cacheHit {
		e.collectGauge(ctx, name)
	}

	// Recorders (always run, errors swallowed).
	recorders := e.recorders
	if len(recorders) == 0 && defaultRecorder != nil {
		recorders = []Recorder{defaultRecorder}
	}
	for _, r := range recorders {
		r.Record(ctx, name, input, output, execErr, elapsed)
	}

	return output, execErr
}

// All delegates to the wrapped executor.
func (e *Envelope) All() []tool.Tool { return e.executor.All() }

// Names delegates to the wrapped executor.
func (e *Envelope) Names() []string { return e.executor.Names() }

// truncateResult applies per-tool or default MaxResultSize truncation to text content.
// StructuredContent is preserved — only Text() output is truncated.
func (e *Envelope) truncateResult(name string, r tool.Result) tool.Result {
	limit := e.maxResultDefault

	// Check per-tool override via ToolMetadata.
	for _, t := range e.executor.All() {
		if t.Name() == name {
			if tm, ok := t.(tool.ToolMetadata); ok {
				if perTool := tm.Metadata().MaxResultSize; perTool > 0 {
					limit = perTool
				}
			}
			break
		}
	}

	if limit <= 0 {
		return r
	}

	text := r.Text()
	if len(text) <= limit {
		return r
	}

	// Replace all TextContent blocks with a single truncated block.
	truncated := text[:limit] + fmt.Sprintf("\n[battery: output truncated, %d bytes of %d limit]", len(text), limit)
	var kept []tool.Content
	for _, c := range r.Content {
		if _, ok := c.(tool.TextContent); !ok {
			kept = append(kept, c)
		}
	}
	kept = append(kept, tool.TextContent{Text: truncated})
	r.Content = kept
	return r
}

// checkCache looks up a cached result if the tool implements Cacheable and a CacheStore is configured.
// Returns the cache key and the cached Result (nil on miss or no cache).
func (e *Envelope) checkCache(ctx context.Context, name string, input json.RawMessage) (string, *tool.Result) {
	if e.cacheStore == nil {
		return "", nil
	}

	// Find the tool and check Cacheable.
	var cacheKey string
	for _, t := range e.executor.All() {
		if t.Name() == name {
			if c, ok := t.(tool.Cacheable); ok {
				key, cacheable := c.CacheKey(ctx, input)
				if cacheable {
					cacheKey = key
				}
			}
			break
		}
	}
	if cacheKey == "" {
		return "", nil
	}

	// Check cache. Deserialize via tool.Result (has MarshalJSON/UnmarshalJSON).
	data, ok, err := e.cacheStore.Get(ctx, name, cacheKey)
	if err != nil || !ok {
		if e.cacheHook != nil {
			e.cacheHook.OnCacheMiss(ctx, name, cacheKey)
		}
		return cacheKey, nil
	}

	var result tool.Result
	if err := json.Unmarshal(data, &result); err != nil {
		return cacheKey, nil
	}

	if e.cacheHook != nil {
		e.cacheHook.OnCacheHit(ctx, name, cacheKey)
	}
	return cacheKey, &result
}

// storeCache serializes and stores a Result in the cache.
func (e *Envelope) storeCache(ctx context.Context, name, key string, r tool.Result) {
	if e.cacheStore == nil {
		return
	}
	data, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = e.cacheStore.Set(ctx, name, key, data, e.cacheTTL) // errors swallowed
}

// collectGauge checks if the executed tool implements Gauged and fires gaugeFunc.
func (e *Envelope) collectGauge(ctx context.Context, name string) {
	for _, t := range e.executor.All() {
		if t.Name() == name {
			if g, ok := t.(tool.Gauged); ok {
				ms := g.LastMeasurement()
				if len(ms) > 0 {
					e.gaugeFunc(ctx, name, ms)
				}
			}
			return
		}
	}
}

// Builder constructs an Envelope with "security by construction" —
// Build() refuses without at least one SecurityGate.
type Builder struct {
	gates            []Gate
	enrichers        []Enricher
	recorders        []Recorder
	executor         tool.Executor
	hasSecurity      bool
	maxResultDefault int
	gaugeFunc        GaugeFunc
	cacheStore       CacheStore
	cacheHook        CacheHook
	cacheTTL         time.Duration
}

// NewBuilder creates an Envelope builder wrapping the given executor.
func NewBuilder(executor tool.Executor) *Builder {
	return &Builder{executor: executor}
}

// WithGate adds a gate. If the gate implements SecurityGate, marks security as satisfied.
func (b *Builder) WithGate(g Gate) *Builder {
	b.gates = append(b.gates, g)
	if _, ok := g.(SecurityGate); ok {
		b.hasSecurity = true
	}
	return b
}

// WithGates adds multiple gates.
func (b *Builder) WithGates(gs ...Gate) *Builder {
	for _, g := range gs {
		b.WithGate(g)
	}
	return b
}

// WithEnricher adds an enricher.
func (b *Builder) WithEnricher(e Enricher) *Builder {
	b.enrichers = append(b.enrichers, e)
	return b
}

// WithEnrichers adds multiple enrichers.
func (b *Builder) WithEnrichers(es ...Enricher) *Builder {
	b.enrichers = append(b.enrichers, es...)
	return b
}

// WithRecorder adds a recorder.
func (b *Builder) WithRecorder(r Recorder) *Builder {
	b.recorders = append(b.recorders, r)
	return b
}

// WithRecorders adds multiple recorders.
func (b *Builder) WithRecorders(rs ...Recorder) *Builder {
	b.recorders = append(b.recorders, rs...)
	return b
}

// WithMaxResultSize sets the default max output size in bytes.
// Tools that implement ToolMetadata can override this per-tool.
// Zero means unlimited (default).
func (b *Builder) WithMaxResultSize(n int) *Builder {
	b.maxResultDefault = n
	return b
}

// WithGaugeFunc sets a callback for tools that implement tool.Gauged.
// After Execute, if the tool reports measurements, fn is called.
// The callback should not block.
func (b *Builder) WithGaugeFunc(fn GaugeFunc) *Builder {
	b.gaugeFunc = fn
	return b
}

// WithCache enables result caching for tools that implement tool.Cacheable.
// The store holds serialized tool.Result values. The hook receives hit/miss events.
// TTL applies to all cached entries (0 = store's default).
func (b *Builder) WithCache(store CacheStore, hook CacheHook, ttl time.Duration) *Builder {
	b.cacheStore = store
	b.cacheHook = hook
	b.cacheTTL = ttl
	return b
}

// Build creates the Envelope. Fails if no SecurityGate was added.
func (b *Builder) Build() (*Envelope, error) {
	if !b.hasSecurity {
		return nil, ErrNoSecurityGate
	}
	return &Envelope{
		gates:            b.gates,
		enrichers:        b.enrichers,
		recorders:        b.recorders,
		executor:         b.executor,
		maxResultDefault: b.maxResultDefault,
		gaugeFunc:        b.gaugeFunc,
		cacheStore:       b.cacheStore,
		cacheHook:        b.cacheHook,
		cacheTTL:         b.cacheTTL,
	}, nil
}
