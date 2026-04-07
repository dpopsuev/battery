package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// ErrNoSecurityGate indicates Build() was called without a SecurityGate.
var ErrNoSecurityGate = errors.New("battery: cannot build Envelope without SecurityGate — security by construction")

// Envelope wraps a tool.Executor with Gate → Enrich → Execute → Record pipeline.
// Implements tool.Executor (LSP — substitutable for any Executor).
type Envelope struct {
	gates     []Gate
	enrichers []Enricher
	recorders []Recorder
	executor  tool.Executor
}

var _ tool.Executor = (*Envelope)(nil)

// Execute runs the full pipeline: check gates → enrich → execute → record.
func (e *Envelope) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	// Gates.
	for _, g := range e.gates {
		v, err := g.Check(ctx, name, input)
		if err != nil {
			return "", fmt.Errorf("%w: gate error: %v", ErrToolDenied, err)
		}
		if !v.Allowed {
			return "", fmt.Errorf("%w: %s", ErrToolDenied, v.Reason)
		}
	}

	// Enrichers (non-fatal, append to output on success).
	var enrichments []string
	for _, en := range e.enrichers {
		result, err := en.Enrich(ctx, name, input)
		if err == nil && result != "" {
			enrichments = append(enrichments, result)
		}
	}

	// Execute.
	start := time.Now()
	output, execErr := e.executor.Execute(ctx, name, input)
	elapsed := time.Since(start)

	// Append enrichments to output on success.
	if execErr == nil && len(enrichments) > 0 {
		output = output + "\n\n" + strings.Join(enrichments, "\n")
	}

	// Recorders (always run, errors swallowed).
	for _, r := range e.recorders {
		r.Record(ctx, name, input, output, execErr, elapsed)
	}

	return output, execErr
}

// All delegates to the wrapped executor.
func (e *Envelope) All() []tool.Tool { return e.executor.All() }

// Names delegates to the wrapped executor.
func (e *Envelope) Names() []string { return e.executor.Names() }

// Builder constructs an Envelope with "security by construction" —
// Build() refuses without at least one SecurityGate.
type Builder struct {
	gates       []Gate
	enrichers   []Enricher
	recorders   []Recorder
	executor    tool.Executor
	hasSecurity bool
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

// Build creates the Envelope. Fails if no SecurityGate was added.
func (b *Builder) Build() (*Envelope, error) {
	if !b.hasSecurity {
		return nil, ErrNoSecurityGate
	}
	return &Envelope{
		gates:     b.gates,
		enrichers: b.enrichers,
		recorders: b.recorders,
		executor:  b.executor,
	}, nil
}
