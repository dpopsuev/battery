// Package workbench provides runtime tool composition — mount multiple
// Executor sources, craft inline tools, build pipelines, and swap tools
// conditionally. Implements tool.Executor (LSP substitutable).
package workbench

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/dpopsuev/battery/tool"
)

// Workbench composes tools from multiple sources into a single Executor.
type Workbench struct {
	sources []tool.Executor
	crafted map[string]tool.Tool
	pipes   map[string][]string // pipeline name → step tool names
	swaps   map[string]SwapRule
}

var _ tool.Executor = (*Workbench)(nil)

// New creates an empty Workbench.
func New() *Workbench {
	return &Workbench{
		crafted: make(map[string]tool.Tool),
		pipes:   make(map[string][]string),
		swaps:   make(map[string]SwapRule),
	}
}

// Mount adds a tool source. All tools from the source become available.
func (w *Workbench) Mount(exec tool.Executor) *Workbench {
	w.sources = append(w.sources, exec)
	return w
}

// Craft registers a single tool directly.
func (w *Workbench) Craft(t tool.Tool) *Workbench {
	w.crafted[t.Name()] = t
	return w
}

// Pipe creates a named pipeline. When executed, the output of step N
// becomes the input of step N+1 as {"input":"<output>"}.
func (w *Workbench) Pipe(name string, steps ...string) *Workbench {
	w.pipes[name] = steps
	return w
}

// Swap registers a conditional tool swap rule.
func (w *Workbench) Swap(rule SwapRule) *Workbench {
	w.swaps[rule.Name] = rule
	return w
}

// Execute dispatches a tool call. Checks pipes first, then swaps, then
// crafted tools, then mounted sources in order.
func (w *Workbench) Execute(ctx context.Context, name string, input json.RawMessage) (tool.Result, error) {
	// Pipeline execution.
	if steps, ok := w.pipes[name]; ok {
		return w.executePipe(ctx, steps, input)
	}

	// Resolve the tool (swap-aware).
	t, err := w.resolve(name)
	if err != nil {
		return tool.Result{}, err
	}
	return t.Execute(ctx, input)
}

// All returns all available tools from all sources, crafted tools,
// and swap-resolved tools. Pipe names are not included.
func (w *Workbench) All() []tool.Tool {
	seen := make(map[string]bool)
	out := make([]tool.Tool, 0, len(w.crafted)+len(w.swaps))

	// Swap-resolved tools first.
	for name, rule := range w.swaps {
		if seen[name] {
			continue
		}
		seen[name] = true
		if rule.Predicate() {
			out = append(out, rule.Primary)
		} else {
			out = append(out, rule.Fallback)
		}
	}

	// Crafted tools.
	for name, t := range w.crafted {
		if seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, t)
	}

	// Mounted sources.
	for _, src := range w.sources {
		for _, t := range src.All() {
			if seen[t.Name()] {
				continue
			}
			seen[t.Name()] = true
			out = append(out, t)
		}
	}

	return out
}

// Names returns all available tool names (including pipe names), sorted.
func (w *Workbench) Names() []string {
	seen := make(map[string]bool)

	for name := range w.pipes {
		seen[name] = true
	}
	for _, t := range w.All() {
		seen[t.Name()] = true
	}

	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// resolve finds a tool by name, checking swaps then crafted then sources.
func (w *Workbench) resolve(name string) (tool.Tool, error) {
	// Check swap rules.
	if rule, ok := w.swaps[name]; ok {
		if rule.Predicate() {
			return rule.Primary, nil
		}
		return rule.Fallback, nil
	}

	// Check crafted tools.
	if t, ok := w.crafted[name]; ok {
		return t, nil
	}

	// Check mounted sources.
	for _, src := range w.sources {
		for _, t := range src.All() {
			if t.Name() == name {
				return t, nil
			}
		}
	}

	return nil, fmt.Errorf("%w: %s", tool.ErrNotFound, name)
}

// executePipe runs a pipeline: output of step N → input of step N+1.
// If step N returns StructuredContent, it is serialized as the input for step N+1.
// Otherwise, the full Result is serialized as {"result": <Result>} for step N+1.
func (w *Workbench) executePipe(ctx context.Context, steps []string, input json.RawMessage) (tool.Result, error) {
	current := input
	var lastResult tool.Result
	for i, step := range steps {
		t, err := w.resolve(step)
		if err != nil {
			return tool.Result{}, fmt.Errorf("pipe step %d (%s): %w", i, step, err)
		}
		lastResult, err = t.Execute(ctx, current)
		if err != nil {
			return tool.Result{}, fmt.Errorf("pipe step %d (%s): %w", i, step, err)
		}
		// Pass full Result as input to next step.
		// Prefer StructuredContent if available (typed data).
		// Fall back to serialized Result (preserves all content types).
		if lastResult.StructuredContent != nil {
			current, err = json.Marshal(lastResult.StructuredContent)
		} else {
			current, err = json.Marshal(lastResult)
		}
		if err != nil {
			return tool.Result{}, fmt.Errorf("pipe step %d (%s): marshal: %w", i, step, err)
		}
	}
	return lastResult, nil
}
