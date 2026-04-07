// Package testkit provides stubs for every Battery interface.
// Every stub records calls for assertion. Every stub has a compile-time interface check.
package testkit

import (
	"context"
	"encoding/json"
	"sort"
	"sync"

	"github.com/dpopsuev/battery/tool"
)

// StubTool implements tool.Tool. Returns configured values, records Execute calls.
type StubTool struct {
	NameVal   string
	DescVal   string
	SchemaVal json.RawMessage
	Result    string
	Err       error

	mu    sync.Mutex
	Calls int
}

var _ tool.Tool = (*StubTool)(nil)

// NewStubTool creates a StubTool with the given name and description.
func NewStubTool(name, desc string) *StubTool {
	return &StubTool{NameVal: name, DescVal: desc, Result: "ok"}
}

func (s *StubTool) Name() string                { return s.NameVal }
func (s *StubTool) Description() string          { return s.DescVal }
func (s *StubTool) InputSchema() json.RawMessage { return s.SchemaVal }

func (s *StubTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls++
	return s.Result, s.Err
}

// StubExecutor implements tool.Executor. Dispatches to registered StubTools.
type StubExecutor struct {
	tools map[string]tool.Tool

	mu    sync.Mutex
	Calls []StubExecuteCall
}

// StubExecuteCall records one Execute invocation.
type StubExecuteCall struct {
	Name  string
	Input json.RawMessage
}

var _ tool.Executor = (*StubExecutor)(nil)

// NewStubExecutor creates a StubExecutor with the given tools.
func NewStubExecutor(tools ...tool.Tool) *StubExecutor {
	m := make(map[string]tool.Tool, len(tools))
	for _, t := range tools {
		m[t.Name()] = t
	}
	return &StubExecutor{tools: m}
}

func (s *StubExecutor) Execute(ctx context.Context, name string, input json.RawMessage) (string, error) {
	s.mu.Lock()
	s.Calls = append(s.Calls, StubExecuteCall{Name: name, Input: input})
	s.mu.Unlock()

	t, ok := s.tools[name]
	if !ok {
		return "", tool.ErrNotFound
	}
	return t.Execute(ctx, input)
}

func (s *StubExecutor) All() []tool.Tool {
	out := make([]tool.Tool, 0, len(s.tools))
	for _, t := range s.tools {
		out = append(out, t)
	}
	return out
}

func (s *StubExecutor) Names() []string {
	out := make([]string, 0, len(s.tools))
	for name := range s.tools {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
