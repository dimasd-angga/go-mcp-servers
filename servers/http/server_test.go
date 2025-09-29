package main

import (
	"testing"
)

func TestNewHTTPServer_Defaults(t *testing.T) {
	t.Setenv("HTTP_DEFAULT_TIMEOUT", "")
	t.Setenv("HTTP_MAX_RESPONSE_SIZE", "")
	t.Setenv("HTTP_ALLOWED_HOSTS", "")
	h, err := NewHTTPServer()
	if err != nil {
		t.Fatal(err)
	}
	if h.maxResponseSize != 5*1024*1024 {
		t.Errorf("default max response: %d", h.maxResponseSize)
	}
}

func TestNewHTTPServer_BadTimeout(t *testing.T) {
	t.Setenv("HTTP_DEFAULT_TIMEOUT", "notanumber")
	if _, err := NewHTTPServer(); err == nil {
		t.Fatal("want error for bad timeout")
	}
}

func TestHostAllowed_EmptyMeansAll(t *testing.T) {
	t.Setenv("HTTP_ALLOWED_HOSTS", "")
	h, _ := NewHTTPServer()
	if err := h.hostAllowed("https://example.com"); err != nil {
		t.Errorf("empty allowlist should allow all, got %v", err)
	}
}

func TestHostAllowed_Allow(t *testing.T) {
	t.Setenv("HTTP_ALLOWED_HOSTS", "api.example.com,httpbin.org")
	h, _ := NewHTTPServer()
	if err := h.hostAllowed("https://api.example.com/x"); err != nil {
		t.Errorf("api.example.com should be allowed: %v", err)
	}
	if err := h.hostAllowed("https://HTTPBIN.ORG/x"); err != nil {
		t.Errorf("case-insensitive match should pass: %v", err)
	}
	if err := h.hostAllowed("https://evil.com/x"); err == nil {
		t.Error("evil.com must be rejected")
	}
}

func TestHostAllowed_RejectsBadURL(t *testing.T) {
	t.Setenv("HTTP_ALLOWED_HOSTS", "x.com")
	h, _ := NewHTTPServer()
	if err := h.hostAllowed("not a url"); err == nil {
		t.Error("bad URL must be rejected")
	}
}

func TestTraversePath_Object(t *testing.T) {
	v := map[string]any{"a": map[string]any{"b": "value"}}
	got, err := traversePath(v, "a.b")
	if err != nil {
		t.Fatal(err)
	}
	if got != "value" {
		t.Errorf("want value, got %v", got)
	}
}

func TestTraversePath_List(t *testing.T) {
	v := map[string]any{"items": []any{"x", "y", "z"}}
	got, err := traversePath(v, "items.1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "y" {
		t.Errorf("want y, got %v", got)
	}
}

func TestTraversePath_Missing(t *testing.T) {
	v := map[string]any{"a": 1}
	if _, err := traversePath(v, "missing"); err == nil {
		t.Error("missing key should error")
	}
}

func TestExtractText_StripsTagsAndScripts(t *testing.T) {
	in := `<html><body><h1>Hello</h1><script>alert(1)</script><p>World</p></body></html>`
	got, err := extractText(in)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Hello World" {
		t.Errorf("want %q, got %q", "Hello World", got)
	}
}
