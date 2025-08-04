// Package auth implements a simple bearer-token check used by MCP servers
// when MCP_AUTH_TOKEN is set in the environment.
package auth

import (
	"crypto/subtle"
	"errors"
	"os"
)

// ErrUnauthorized is returned when the supplied token does not match.
var ErrUnauthorized = errors.New("invalid auth token")

// Validate checks providedToken against the MCP_AUTH_TOKEN env var.
// If the env var is empty, auth is disabled and Validate returns nil.
// Otherwise it compares in constant time.
func Validate(providedToken string) error {
	expected := os.Getenv("MCP_AUTH_TOKEN")
	if expected == "" {
		return nil
	}
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(expected)) != 1 {
		return ErrUnauthorized
	}
	return nil
}
