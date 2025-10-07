package main

import (
	"testing"
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
