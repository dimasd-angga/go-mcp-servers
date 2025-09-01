package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemServer) registerGetFileInfo() {
	fs.mcp.AddTool(
		mcp.NewTool("get_file_info",
			mcp.WithDescription("Stat a path under FS_ROOT. Returns JSON with type, size, mtime, mode."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to FS_ROOT")),
		),
		fs.getFileInfo,
	)
}

type fileInfo struct {
	Path  string `json:"path"`
	Type  string `json:"type"`
	Size  int64  `json:"size"`
	Mtime string `json:"mtime"`
	Mode  string `json:"mode"`
}

func (fs *FilesystemServer) getFileInfo(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := req.GetArguments()["path"].(string)
	full, err := fs.safePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	info, err := os.Stat(full)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stat: %v", err)), nil
	}
	out := fileInfo{
		Path:  path,
		Size:  info.Size(),
		Mtime: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
		Mode:  info.Mode().String(),
	}
	if info.IsDir() {
		out.Type = "directory"
	} else if info.Mode()&os.ModeSymlink != 0 {
		out.Type = "symlink"
	} else {
		out.Type = "file"
	}
	body, _ := json.MarshalIndent(out, "", "  ")
	return mcp.NewToolResultText(string(body)), nil
}
