package auth

import (
	"errors"
	"testing"
)

func TestValidate_AuthDisabledWhenEnvUnset(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN", "")
	if err := Validate("anything"); err != nil {
		t.Errorf("want nil when MCP_AUTH_TOKEN unset, got %v", err)
	}
}

func TestValidate_AcceptsMatching(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN", "secret123")
	if err := Validate("secret123"); err != nil {
		t.Errorf("want nil for matching token, got %v", err)
	}
}

func TestValidate_RejectsMismatch(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN", "secret123")
	err := Validate("wrong")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
}

func TestValidate_RejectsEmptyWhenTokenSet(t *testing.T) {
	t.Setenv("MCP_AUTH_TOKEN", "secret123")
	err := Validate("")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized for empty, got %v", err)
	}
}
