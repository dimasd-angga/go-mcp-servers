package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fsrv *FilesystemServer) registerListDirectory() {
	fsrv.mcp.AddTool(
		mcp.NewTool("list_directory",
			mcp.WithDescription("List entries in a directory under FS_ROOT. "+
				"Directories are suffixed with '/'. Use recursive=true to walk subdirectories."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Directory relative to FS_ROOT. Use '.' for root.")),
			mcp.WithBoolean("recursive", mcp.Description("Walk subdirectories. Default false.")),
		),
		fsrv.listDirectory,
	)
}

func (fsrv *FilesystemServer) listDirectory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	path, _ := args["path"].(string)
	if path == "" {
		path = "."
	}
	recursive, _ := args["recursive"].(bool)
	full, err := fsrv.safePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if info, err := os.Stat(full); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stat: %v", err)), nil
	} else if !info.IsDir() {
		return mcp.NewToolResultError(fmt.Sprintf("%s is not a directory", path)), nil
	}

	var entries []string
	if recursive {
		walkErr := filepath.WalkDir(full, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if p == full {
				return nil
			}
			rel, _ := filepath.Rel(full, p)
			if d.IsDir() {
				entries = append(entries, rel+"/")
			} else {
				entries = append(entries, rel)
			}
			return nil
		})
		if walkErr != nil {
			return mcp.NewToolResultError(fmt.Sprintf("walk: %v", walkErr)), nil
		}
	} else {
		dirEntries, err := os.ReadDir(full)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("readdir: %v", err)), nil
		}
		for _, e := range dirEntries {
			name := e.Name()
			if e.IsDir() {
				name += "/"
			}
			entries = append(entries, name)
		}
	}
	sort.Strings(entries)
	if len(entries) == 0 {
		return mcp.NewToolResultText("(empty)"), nil
	}
	return mcp.NewToolResultText(strings.Join(entries, "\n")), nil
}
