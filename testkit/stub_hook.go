package testkit

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/dpopsuev/battery/observer"
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

// StubHook implements observer.Hook with call recording.
type StubHook struct {
	mu          sync.Mutex
	ToolCalls   []StubHookToolCall
	ToolResults []StubHookToolResult
}

var _ observer.Hook = (*StubHook)(nil)

func (h *StubHook) OnToolCall(_ context.Context, tool string, input json.RawMessage) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolCalls = append(h.ToolCalls, StubHookToolCall{Tool: tool, Input: input})
}

func (h *StubHook) OnToolResult(_ context.Context, tool, output string, err error, elapsed time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ToolResults = append(h.ToolResults, StubHookToolResult{Tool: tool, Output: output, Err: err, Elapsed: elapsed})
}
