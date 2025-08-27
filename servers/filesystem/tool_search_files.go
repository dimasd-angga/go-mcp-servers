package main

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fsrv *FilesystemServer) registerSearchFiles() {
	fsrv.mcp.AddTool(
		mcp.NewTool("search_files",
			mcp.WithDescription("Find files under FS_ROOT matching a glob pattern. "+
				"Patterns are matched against both relative path and filename "+
				"(so '*.go' and 'src/*.go' both work). Returns one path per line."),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Glob pattern, e.g. '*.go'")),
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
	if _, err := filepath.Match(pattern, ""); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid pattern: %v", err)), nil
	}

	var matches []string
	err := filepath.WalkDir(fsrv.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if p == fsrv.root {
			return nil
		}
		rel, _ := filepath.Rel(fsrv.root, p)
		pathMatch, _ := filepath.Match(pattern, rel)
		nameMatch, _ := filepath.Match(pattern, d.Name())
		if pathMatch || nameMatch {
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
