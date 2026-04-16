package tool

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
)

// ErrToolNotAllowed indicates a tool is not available for the current role.
var ErrToolNotAllowed = errors.New("battery: tool not available for role")

// Clearance wraps a tool.Executor with role-based filtering.
// Only tools in the allowlist are visible and executable.
// Implements Executor (LSP — substitutable).
type Clearance struct {
	executor Executor
	allowed  map[string]bool
}

var _ Executor = (*Clearance)(nil)

// NewClearance creates a Clearance that filters tools by an allowlist.
// If allowedTools is empty, all tools are allowed.
func NewClearance(executor Executor, allowedTools []string) *Clearance {
	allowed := make(map[string]bool, len(allowedTools))
	for _, name := range allowedTools {
		allowed[name] = true
	}
	return &Clearance{executor: executor, allowed: allowed}
}

// Execute runs a tool if it's in the allowlist.
func (c *Clearance) Execute(ctx context.Context, name string, input json.RawMessage) (Result, error) {
	if len(c.allowed) > 0 && !c.allowed[name] {
		return Result{}, ErrToolNotAllowed
	}
	return c.executor.Execute(ctx, name, input)
}

// All returns only the allowed tools.
func (c *Clearance) All() []Tool {
	all := c.executor.All()
	if len(c.allowed) == 0 {
		return all
	}
	var out []Tool
	for _, t := range all {
		if c.allowed[t.Name()] {
			out = append(out, t)
		}
	}
	return out
}

// Names returns only the allowed tool names, sorted.
func (c *Clearance) Names() []string {
	all := c.executor.Names()
	if len(c.allowed) == 0 {
		return all
	}
	var out []string
	for _, name := range all {
		if c.allowed[name] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}
