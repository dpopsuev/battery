package testkit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/dpopsuev/battery/observer"
	"github.com/dpopsuev/battery/tool"
)

// StubHookToolCall records one OnToolCall invocation.
type StubHookToolCall struct {
	Tool  string
	Input json.RawMessage
}

// StubHookToolResult records one OnToolResult invocation.
type StubHookToolResult struct {
	Tool    string
	Output  string
	Err     error
	Elapsed time.Duration
}

// StubGaugeMeasurement records one OnGaugeMeasurement invocation.
type StubGaugeMeasurement struct {
	Tool         string
	Measurements []tool.Measurement
}

// StubHook implements observer.Hook with call recording.
type StubHook struct {
	mu                sync.Mutex
	ToolCalls         []StubHookToolCall
	ToolResults       []StubHookToolResult
	GaugeMeasurements []StubGaugeMeasurement
}

var _ observer.Hook = (*StubHook)(nil)

func (h *StubHook) OnToolCall(_ context.Context, tool string, input json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCalls = append(h.ToolCalls, StubHookToolCall{Tool: tool, Input: input})
}

func (h *StubHook) OnToolResult(_ context.Context, toolName, output string, err error, elapsed time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolResults = append(h.ToolResults, StubHookToolResult{Tool: toolName, Output: output, Err: err, Elapsed: elapsed})
}

func (h *StubHook) OnGaugeMeasurement(_ context.Context, toolName string, measurements []tool.Measurement) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.GaugeMeasurements = append(h.GaugeMeasurements, StubGaugeMeasurement{Tool: toolName, Measurements: measurements})
}
