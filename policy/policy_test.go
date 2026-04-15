package policy_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dpopsuev/battery/policy"
)

func TestCapabilityToken_ZeroValuePermissive(t *testing.T) {
	token := policy.CapabilityToken{}
	if len(token.AllowedTools) != 0 {
		t.Error("zero value should have empty AllowedTools (all allowed)")
	}
	if len(token.WritablePaths) != 0 {
		t.Error("zero value should have empty WritablePaths")
	}
	if len(token.DeniedPaths) != 0 {
		t.Error("zero value should have empty DeniedPaths")
	}
	if token.Tier != "" {
		t.Error("zero value should have empty Tier")
	}
}

func TestCapabilityToken_Fields(t *testing.T) {
	token := policy.CapabilityToken{
		WritablePaths: []string{"/home/user/project"},
		DeniedPaths:   []string{"/etc", "/usr"},
		AllowedTools:  []string{"read", "write"},
		Tier:          "sys",
	}
	if len(token.WritablePaths) != 1 || token.WritablePaths[0] != "/home/user/project" {
		t.Errorf("WritablePaths = %v", token.WritablePaths)
	}
	if len(token.DeniedPaths) != 2 {
		t.Errorf("DeniedPaths = %v", token.DeniedPaths)
	}
	if len(token.AllowedTools) != 2 || token.AllowedTools[0] != "read" {
		t.Errorf("AllowedTools = %v", token.AllowedTools)
	}
	if token.Tier != "sys" {
		t.Errorf("Tier = %q", token.Tier)
	}
}

// EnforcerContract validates any Enforcer implementation.
func EnforcerContract(t *testing.T, newEnforcer func(err error) policy.Enforcer) {
	t.Helper()

	t.Run("AllowsWhenNoError", func(t *testing.T) {
		e := newEnforcer(nil)
		err := e.Check(context.Background(), policy.CapabilityToken{}, "read", nil)
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	})

	t.Run("DeniesWhenError", func(t *testing.T) {
		denied := errors.New("access denied")
		e := newEnforcer(denied)
		err := e.Check(context.Background(), policy.CapabilityToken{}, "write", nil)
		if !errors.Is(err, denied) {
			t.Errorf("expected %v, got %v", denied, err)
		}
	})

	t.Run("PassesTokenAndTool", func(t *testing.T) {
		e := newEnforcer(nil)
		token := policy.CapabilityToken{AllowedTools: []string{"read"}, Tier: "eco"}
		err := e.Check(context.Background(), token, "read", json.RawMessage(`{"path":"."}`))
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// stubEnforcer is a minimal Enforcer for contract testing.
type stubEnforcer struct{ err error }

func (e *stubEnforcer) Check(_ context.Context, _ policy.CapabilityToken, _ string, _ json.RawMessage) error {
	return e.err
}

func TestStubEnforcer_Contract(t *testing.T) {
	EnforcerContract(t, func(err error) policy.Enforcer {
		return &stubEnforcer{err: err}
	})
}
