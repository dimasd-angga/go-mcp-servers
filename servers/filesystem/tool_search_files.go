package main

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/mark3labs/mcp-go/mcp"
)

func (fsrv *FilesystemServer) registerSearchFiles() {
	fsrv.mcp.AddTool(
		mcp.NewTool("search_files",
			mcp.WithDescription("Find files under FS_ROOT matching a glob pattern. "+
				"Supports multi-segment globs (e.g. '**/*.go' matches Go files at any depth). "+
				"Patterns are matched against the relative path of each file. Returns one path per line."),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern, e.g. '**/*.go' or 'src/*.go'")),
		),
		fsrv.searchFiles,
	)
}

func (fsrv *FilesystemServer) searchFiles(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	pattern, _ := req.GetArguments()["pattern"].(string)
	if pattern == "" {
		return mcp.NewToolResultError("pattern is required"), nil
	}
	// Validate the pattern by running it once.
	if _, err := doublestar.Match(pattern, ""); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid pattern: %v", err)), nil
	}

	var matches []string
	err := filepath.WalkDir(fsrv.root, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == fsrv.root {
			return nil
		}
		rel, _ := filepath.Rel(fsrv.root, p)
		rel = filepath.ToSlash(rel)
		matched, _ := doublestar.Match(pattern, rel)
		if matched {
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("walk: %v", err)), nil
	}
	if len(matches) == 0 {
		return mcp.NewToolResultText("(no files matched)"), nil
	}
	sort.Strings(matches)
	return mcp.NewToolResultText(strings.Join(matches, "\n")), nil
}
