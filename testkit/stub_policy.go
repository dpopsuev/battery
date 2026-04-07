package testkit

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/dpopsuev/battery/policy"
)

// StubEnforcer implements policy.Enforcer. Returns configured error.
type StubEnforcer struct {
	Err error

	mu    sync.Mutex
	Calls int
	Last  StubEnforcerCall
}

// StubEnforcerCall records one Check call.
type StubEnforcerCall struct {
	Token policy.CapabilityToken
	Tool  string
	Input json.RawMessage
}

var _ policy.Enforcer = (*StubEnforcer)(nil)

// AllowAllToken returns a CapabilityToken with no restrictions.
func AllowAllToken() policy.CapabilityToken {
	return policy.CapabilityToken{}
}

func (e *StubEnforcer) Check(_ context.Context, token policy.CapabilityToken, tool string, input json.RawMessage) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.Calls++
	e.Last = StubEnforcerCall{Token: token, Tool: tool, Input: input}
	return e.Err
}
