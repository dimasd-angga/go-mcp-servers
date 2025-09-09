package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

// ShellServer runs allowed shell commands inside a sandboxed working directory
// with output truncation and an explicit denylist of catastrophic patterns.
type ShellServer struct {
	mcp             *server.MCPServer
	workdir         string
	allowedCmds     []string
	envPassthrough  []string
	timeoutSeconds  int
	maxOutputBytes  int
}

func (s *ShellServer) MCP() *server.MCPServer { return s.mcp }
func (s *ShellServer) Workdir() string         { return s.workdir }
func (s *ShellServer) Timeout() int            { return s.timeoutSeconds }
func (s *ShellServer) MaxOutput() int          { return s.maxOutputBytes }
func (s *ShellServer) AllowedCmds() []string   { return s.allowedCmds }

// NewShellServer builds the server from environment configuration:
//
//	SHELL_WORKDIR         required, must exist
//	SHELL_ALLOWED_CMDS    optional, comma-separated command prefixes
//	SHELL_TIMEOUT         optional, seconds, default 60
//	SHELL_MAX_OUTPUT      optional, bytes, default 50KB
//	SHELL_ENV_PASSTHROUGH optional, comma-separated env names, default "PATH"
func NewShellServer() (*ShellServer, error) {
	workdir := os.Getenv("SHELL_WORKDIR")
	if workdir == "" {
		return nil, fmt.Errorf("SHELL_WORKDIR environment variable is required")
	}
	absWorkdir, err := filepath.Abs(workdir)
	if err != nil {
		return nil, fmt.Errorf("invalid SHELL_WORKDIR: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absWorkdir); err == nil {
		absWorkdir = resolved
	}
	info, err := os.Stat(absWorkdir)
	if err != nil {
		return nil, fmt.Errorf("SHELL_WORKDIR does not exist: %s", absWorkdir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("SHELL_WORKDIR is not a directory: %s", absWorkdir)
	}

	timeout := 60
	if v := os.Getenv("SHELL_TIMEOUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid SHELL_TIMEOUT %q", v)
		}
		timeout = n
	}
	maxOutput := 50 * 1024
	if v := os.Getenv("SHELL_MAX_OUTPUT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid SHELL_MAX_OUTPUT %q", v)
		}
		maxOutput = n
	}

	var allowed []string
	if v := strings.TrimSpace(os.Getenv("SHELL_ALLOWED_CMDS")); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				allowed = append(allowed, p)
			}
		}
	}

	passthrough := []string{"PATH"}
	if v := strings.TrimSpace(os.Getenv("SHELL_ENV_PASSTHROUGH")); v != "" {
		passthrough = passthrough[:0]
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				passthrough = append(passthrough, p)
			}
		}
	}

	mcp := server.NewMCPServer("shell", "1.0.0", server.WithToolCapabilities(true))
	s := &ShellServer{
		mcp:            mcp,
		workdir:        absWorkdir,
		allowedCmds:    allowed,
		envPassthrough: passthrough,
		timeoutSeconds: timeout,
		maxOutputBytes: maxOutput,
	}
	s.registerTools()
	return s, nil
}

// buildEnv returns the child-process environment containing only variables
// named in SHELL_ENV_PASSTHROUGH.
func (s *ShellServer) buildEnv() []string {
	env := make([]string, 0, len(s.envPassthrough))
	for _, name := range s.envPassthrough {
		if v, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+v)
		}
	}
	return env
}

// dangerPatterns matches catastrophic commands the server refuses outright.
// Keep the list short and conservative; the allowlist is the primary defense.
var dangerPatterns = []string{
	"rm -rf /",
	"rm -rf /*",
	":(){",
	"> /dev/sd",
	"> /dev/nvme",
	"mkfs",
	"dd if=/dev/zero of=/dev/",
	"shutdown ",
	"reboot",
}

func (s *ShellServer) preflight(cmd string) (rejection string, ok bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "empty command", false
	}
	for _, p := range dangerPatterns {
		if strings.Contains(trimmed, p) {
			return "command rejected: dangerous pattern detected", false
		}
	}
	if len(s.allowedCmds) > 0 {
		allowed := false
		for _, prefix := range s.allowedCmds {
			if strings.HasPrefix(trimmed, prefix) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "command not in SHELL_ALLOWED_CMDS", false
		}
	}
	return "", true
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)

func stripAnsi(s string) string { return ansiRe.ReplaceAllString(s, "") }

func truncate(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	return s[:limit] + fmt.Sprintf("\n[output truncated at %d bytes]", limit)
}
