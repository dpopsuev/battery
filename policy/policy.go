// Package policy defines capability tokens and enforcement for agent tool access.
package policy

import (
	"context"
	"encoding/json"
)

// CapabilityToken declares what an agent is allowed to do.
// Zero value means no restrictions (all tools allowed, all paths writable).
type CapabilityToken struct {
	WritablePaths []string // filesystem paths the agent can write to
	DeniedPaths   []string // paths always denied
	AllowedTools  []string // tool whitelist (empty = all allowed)
	Tier          string   // privilege level (eco, sys, com, mod)
}

// Enforcer checks tool calls against a capability token.
type Enforcer interface {
	Check(ctx context.Context, token CapabilityToken, tool string, input json.RawMessage) error
}
