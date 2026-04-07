package middleware

import (
	"context"
	"encoding/json"

	"github.com/dpopsuev/battery/policy"
)

// PolicyGate implements SecurityGate by delegating to a policy.Enforcer.
type PolicyGate struct {
	enforcer policy.Enforcer
	token    policy.CapabilityToken
}

// NewPolicyGate creates a SecurityGate that checks tool calls against a capability token.
func NewPolicyGate(enforcer policy.Enforcer, token policy.CapabilityToken) SecurityGate {
	return AsSecurityGate(&PolicyGate{enforcer: enforcer, token: token})
}

// Check delegates to the enforcer.
func (g *PolicyGate) Check(ctx context.Context, toolName string, input json.RawMessage) (Verdict, error) {
	err := g.enforcer.Check(ctx, g.token, toolName, input)
	if err != nil {
		return Verdict{Allowed: false, Reason: err.Error()}, nil
	}
	return Verdict{Allowed: true}, nil
}
