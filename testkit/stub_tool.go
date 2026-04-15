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
	Result    string // convenience: used by Execute to build tool.TextResult
	Err       error

	mu    sync.Mutex
	Calls int
}

var _ tool.Tool = (*StubTool)(nil)

// NewStubTool creates a StubTool with the given name and description.
func NewStubTool(name, desc string) *StubTool {
	return &StubTool{NameVal: name, DescVal: desc, Result: "ok"}
}

func (s *StubTool) Name() string                 { return s.NameVal }
func (s *StubTool) Description() string          { return s.DescVal }
func (s *StubTool) InputSchema() json.RawMessage { return s.SchemaVal }

func (s *StubTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Calls++
	if s.Err != nil {
		return tool.Result{}, s.Err
	}
	return tool.TextResult(s.Result), nil
}

// StubCapableTool extends StubTool with CapabilityDeclarer.
type StubCapableTool struct {
	StubTool
	Capabilities []string
}

var _ tool.CapabilityDeclarer = (*StubCapableTool)(nil)

// RequiredCapabilities returns the configured capabilities.
func (s *StubCapableTool) RequiredCapabilities() []string { return s.Capabilities }

// StubMetadataTool extends StubTool with ToolMetadata.
type StubMetadataTool struct {
	StubTool
	Meta tool.Metadata
}

var _ tool.ToolMetadata = (*StubMetadataTool)(nil)

// Metadata returns the configured metadata.
func (s *StubMetadataTool) Metadata() tool.Metadata { return s.Meta }

// StubAvailableTool extends StubTool with Availability.
type StubAvailableTool struct {
	StubTool
	IsAvailable bool
}

var _ tool.Availability = (*StubAvailableTool)(nil)

// Available returns the configured availability.
func (s *StubAvailableTool) Available() bool { return s.IsAvailable }

// StubGaugedTool extends StubTool with Gauged.
type StubGaugedTool struct {
	StubTool
	Measurements []tool.Measurement
}

var _ tool.Gauged = (*StubGaugedTool)(nil)

// LastMeasurement returns the configured measurements.
func (s *StubGaugedTool) LastMeasurement() []tool.Measurement { return s.Measurements }

// StubCacheableTool extends StubTool with Cacheable.
type StubCacheableTool struct {
	StubTool
	KeyFn func(json.RawMessage) (string, bool)
}

var _ tool.Cacheable = (*StubCacheableTool)(nil)

// CacheKey delegates to the configured function.
func (s *StubCacheableTool) CacheKey(_ context.Context, input json.RawMessage) (string, bool) {
	if s.KeyFn == nil {
		return "", false
	}
	return s.KeyFn(input)
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

func (s *StubExecutor) Execute(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
	s.mu.Lock()
	s.Calls = append(s.Calls, StubExecuteCall{Name: name, Input: input})
	s.mu.Unlock()

	t, ok := s.tools[name]
	if !ok {
		return tool.Result{}, tool.ErrNotFound
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
