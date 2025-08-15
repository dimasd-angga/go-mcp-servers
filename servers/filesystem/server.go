package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/server"
)

// FilesystemServer is the MCP server for sandboxed filesystem operations.
// All paths are resolved relative to root and validated to never escape it.
type FilesystemServer struct {
	mcp         *server.MCPServer
	root        string
	maxSize     int64
	allowDelete bool
}

// MCP returns the underlying MCP server so callers can serve it over their
// chosen transport.
func (fs *FilesystemServer) MCP() *server.MCPServer { return fs.mcp }

// Root returns the absolute, canonical root path the server is sandboxed to.
func (fs *FilesystemServer) Root() string { return fs.root }

// MaxSize returns the configured max bytes for read/write operations.
func (fs *FilesystemServer) MaxSize() int64 { return fs.maxSize }

// AllowDelete reports whether destructive tools are enabled.
func (fs *FilesystemServer) AllowDelete() bool { return fs.allowDelete }

// NewFilesystemServer builds the server from environment configuration:
//
//	FS_ROOT           required, must exist
//	FS_MAX_FILE_SIZE  optional, bytes, default 10MB
//	FS_ALLOW_DELETE   optional, "true" to enable delete
func NewFilesystemServer() (*FilesystemServer, error) {
	root := os.Getenv("FS_ROOT")
	if root == "" {
		return nil, fmt.Errorf("FS_ROOT environment variable is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("invalid FS_ROOT: %w", err)
	}
	// Resolve symlinks for the root so safePath comparisons are stable.
	if resolved, err := filepath.EvalSymlinks(absRoot); err == nil {
		absRoot = resolved
	}
	info, err := os.Stat(absRoot)
	if err != nil {
		return nil, fmt.Errorf("FS_ROOT does not exist or is not accessible: %s", absRoot)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("FS_ROOT is not a directory: %s", absRoot)
	}

	maxSize := int64(10 * 1024 * 1024)
	if v := os.Getenv("FS_MAX_FILE_SIZE"); v != "" {
		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid FS_MAX_FILE_SIZE %q: must be positive integer", v)
		}
		maxSize = n
	}

	s := server.NewMCPServer("filesystem", "1.0.0", server.WithToolCapabilities(true))
	fs := &FilesystemServer{
		mcp:         s,
		root:        absRoot,
		maxSize:     maxSize,
		allowDelete: os.Getenv("FS_ALLOW_DELETE") == "true",
	}
	fs.registerTools()
	return fs, nil
}

// safePath resolves the user-supplied path against the sandbox root and
// guarantees the result is contained inside it. Absolute paths are treated
// as root-relative (the leading slash is stripped).
func (fs *FilesystemServer) safePath(p string) (string, error) {
	if p == "" {
		return fs.root, nil
	}
	if filepath.IsAbs(p) {
		p = strings.TrimPrefix(p, string(filepath.Separator))
	}
	full := filepath.Clean(filepath.Join(fs.root, p))
	// Use rel rather than HasPrefix to defeat partial-match attacks
	// (e.g., root /tmp/a vs target /tmp/abc).
	rel, err := filepath.Rel(fs.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes root: %s", p)
	}
	return full, nil
}
