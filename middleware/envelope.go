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
	maxResultDefault int       // default max output bytes; 0 = unlimited
	gaugeFunc        GaugeFunc // optional; nil = no gauge collection
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

	// Execute.
	start := time.Now()
	output, execErr := e.executor.Execute(ctx, name, input)
	elapsed := time.Since(start)

	// Append enrichments to result on success.
	if execErr == nil && len(enrichments) > 0 {
		output.Content = append(output.Content, enrichments...)
	}

	// Truncate text output if MaxResultSize is configured.
	if execErr == nil {
		output = e.truncateResult(name, output)
	}

	// Gauge (optional, non-blocking, errors swallowed).
	if execErr == nil && e.gaugeFunc != nil {
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
	}, nil
}
