package main

import (
	"strconv"
	"testing"
	"time"
)

func TestNewHAServer_RequiresURL(t *testing.T) {
	t.Setenv("HA_URL", "")
	t.Setenv("HA_TOKEN", "x")
	if _, err := NewHAServer(); err == nil {
		t.Fatal("expected error for empty HA_URL")
	}
}

func TestNewHAServer_RequiresToken(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123")
	t.Setenv("HA_TOKEN", "")
	if _, err := NewHAServer(); err == nil {
		t.Fatal("expected error for empty HA_TOKEN")
	}
}

func TestNewHAServer_BadTimeout(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123")
	t.Setenv("HA_TOKEN", "x")
	t.Setenv("HA_TIMEOUT", "nope")
	if _, err := NewHAServer(); err == nil {
		t.Fatal("expected error for bad timeout")
	}
}

func TestNewHAServer_BadStatesCacheTTL(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123")
	t.Setenv("HA_TOKEN", "x")
	t.Setenv("HA_STATES_CACHE_TTL", "nope")
	if _, err := NewHAServer(); err == nil {
		t.Fatal("expected error for bad states cache TTL")
	}
}

func TestNewHAServer_RejectsOverflowingStatesCacheTTL(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123")
	t.Setenv("HA_TOKEN", "x")
	t.Setenv("HA_STATES_CACHE_TTL", strconv.FormatInt(maxStatesCacheTTLSeconds+1, 10))
	if _, err := NewHAServer(); err == nil {
		t.Fatal("expected error for overflowing states cache TTL")
	}
}

func TestNewHAServer_DefaultStatesCacheTTL(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123")
	t.Setenv("HA_TOKEN", "x")
	t.Setenv("HA_STATES_CACHE_TTL", "")
	h, err := NewHAServer()
	if err != nil {
		t.Fatal(err)
	}
	if h.statesCacheTTL != 5*time.Second {
		t.Fatalf("states cache TTL = %s, want 5s", h.statesCacheTTL)
	}
}

func TestNewHAServer_StripsTrailingSlash(t *testing.T) {
	t.Setenv("HA_URL", "http://ha.local:8123/")
	t.Setenv("HA_TOKEN", "x")
	t.Setenv("HA_TIMEOUT", "")
	h, err := NewHAServer()
	if err != nil {
		t.Fatal(err)
	}
	if h.BaseURL() != "http://ha.local:8123" {
		t.Errorf("base url should have trailing slash stripped, got %q", h.BaseURL())
	}
}
