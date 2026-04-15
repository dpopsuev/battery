// Package middleware defines the Gate/Enrich/Execute/Record pipeline for tool calls.
package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/dpopsuev/battery/tool"
)

// ErrToolDenied indicates a tool call was rejected by a Gate.
var ErrToolDenied = errors.New("battery: tool denied")

// Gate decides whether a tool call is allowed.
type Gate interface {
	Check(ctx context.Context, tool string, input json.RawMessage) (Verdict, error)
}

// Verdict is the result of a Gate check.
type Verdict struct {
	Allowed bool
	Reason  string
}

// SecurityGate is a Gate that the Envelope Builder requires.
// At least one SecurityGate must be present to build an Envelope.
type SecurityGate interface {
	Gate
	isSecurityGate()
}

// Enricher injects context before tool execution.
type Enricher interface {
	Enrich(ctx context.Context, tool string, input json.RawMessage) (string, error)
}

// securityGateAdapter wraps any Gate as a SecurityGate.
type securityGateAdapter struct{ Gate }

func (securityGateAdapter) isSecurityGate() {}

// AsSecurityGate wraps a Gate as a SecurityGate. Use for testing or
// when the gate implementation lives outside the middleware package.
func AsSecurityGate(g Gate) SecurityGate {
	return securityGateAdapter{g}
}

// GaugeFunc receives measurements from tools that implement tool.Gauged.
// Called by the Envelope after Execute when configured via WithGaugeFunc.
// Defined as a function type to avoid coupling middleware to observer.
type GaugeFunc func(ctx context.Context, tool string, measurements []tool.Measurement)

// Recorder observes after tool execution.
type Recorder interface {
	Record(ctx context.Context, tool string, input json.RawMessage, output string, err error, elapsed time.Duration)
}

// defaultRecorder is the package-level recorder that fires for every
// Envelope.Execute call when no explicit recorders are configured.
// Set once at process startup via SetDefaultRecorder.
var defaultRecorder Recorder //nolint:gochecknoglobals // intentional global, like slog.SetDefault

// SetDefaultRecorder sets the package-level default recorder.
// Pass nil to clear.
func SetDefaultRecorder(r Recorder) { defaultRecorder = r }

// DefaultRecorder returns the current default recorder (nil if not set).
func DefaultRecorder() Recorder { return defaultRecorder }
