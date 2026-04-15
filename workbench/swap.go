package workbench

import "github.com/dpopsuev/battery/tool"

// SwapRule defines a conditional tool swap based on a runtime predicate.
// When the Workbench resolves a tool by name matching a SwapRule, it uses
// Primary if Predicate returns true, otherwise Fallback.
type SwapRule struct {
	Name      string      // tool name this rule applies to
	Predicate func() bool // returns true → use Primary, false → use Fallback
	Primary   tool.Tool
	Fallback  tool.Tool
}
