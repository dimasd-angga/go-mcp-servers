package main

import (
	"os"
	"strings"
	"testing"
)

func TestNewShellServer_RequiresWorkdir(t *testing.T) {
	t.Setenv("SHELL_WORKDIR", "")
	if _, err := NewShellServer(); err == nil {
		t.Fatal("expected error for empty SHELL_WORKDIR")
	}
}

func TestNewShellServer_RejectsNonexistent(t *testing.T) {
	t.Setenv("SHELL_WORKDIR", "/nope-mcp-test-xyz")
	if _, err := NewShellServer(); err == nil {
		t.Fatal("expected error for nonexistent workdir")
	}
}

func TestNewShellServer_HappyPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	t.Setenv("SHELL_TIMEOUT", "")
	t.Setenv("SHELL_MAX_OUTPUT", "")
	t.Setenv("SHELL_ALLOWED_CMDS", "")
	t.Setenv("SHELL_ENV_PASSTHROUGH", "")
	s, err := NewShellServer()
	if err != nil {
		t.Fatal(err)
	}
	if s.Timeout() != 60 {
		t.Errorf("default timeout: %d", s.Timeout())
	}
	if s.MaxOutput() != 50*1024 {
		t.Errorf("default max output: %d", s.MaxOutput())
	}
}

func TestNewShellServer_BadTimeout(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	t.Setenv("SHELL_TIMEOUT", "abc")
	if _, err := NewShellServer(); err == nil {
		t.Fatal("expected error for bad timeout")
	}
}

func TestPreflight_RejectsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	s, _ := NewShellServer()
	if _, ok := s.preflight(""); ok {
		t.Error("empty command must be rejected")
	}
}

func TestPreflight_RejectsDangerous(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	s, _ := NewShellServer()
	for _, c := range []string{
		"rm -rf /",
		"echo hi; rm -rf /*",
		":(){ :|:& };:",
		"dd if=/dev/zero of=/dev/sda",
		"mkfs.ext4 /dev/sda",
		"shutdown -h now",
	} {
		if _, ok := s.preflight(c); ok {
			t.Errorf("must reject: %q", c)
		}
	}
}

func TestPreflight_AllowlistEnforced(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	t.Setenv("SHELL_ALLOWED_CMDS", "echo,ls")
	s, _ := NewShellServer()
	if _, ok := s.preflight("echo hello"); !ok {
		t.Error("echo should be allowed")
	}
	if _, ok := s.preflight("rm anything"); ok {
		t.Error("rm should be blocked by allowlist")
	}
}

func TestPreflight_AllowlistEmptyMeansAll(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	t.Setenv("SHELL_ALLOWED_CMDS", "")
	s, _ := NewShellServer()
	if _, ok := s.preflight("anything goes"); !ok {
		t.Error("empty allowlist should allow everything (still subject to denylist)")
	}
}

func TestBuildEnv_OnlyPassthrough(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SHELL_WORKDIR", dir)
	t.Setenv("SHELL_ENV_PASSTHROUGH", "PATH,FOO")
	t.Setenv("FOO", "bar")
	t.Setenv("SECRET", "must-not-leak")
	s, _ := NewShellServer()
	env := s.buildEnv()
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "FOO=bar") {
		t.Errorf("FOO should pass through, env=%v", env)
	}
	if strings.Contains(joined, "SECRET=") {
		t.Errorf("SECRET should NOT pass through, env=%v", env)
	}
}

func TestStripAnsi(t *testing.T) {
	in := "\x1b[31mred\x1b[0m text"
	want := "red text"
	if got := stripAnsi(in); got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); !strings.HasPrefix(got, "hello") || !strings.Contains(got, "truncated") {
		t.Errorf("truncate failed: %q", got)
	}
	if got := truncate("ok", 100); got != "ok" {
		t.Errorf("should not truncate small input: %q", got)
	}
	_ = os.Getenv // keep import used in test scope
}
