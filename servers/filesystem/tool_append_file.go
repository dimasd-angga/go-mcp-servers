package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemServer) registerAppendFile() {
	fs.mcp.AddTool(
		mcp.NewTool("append_file",
			mcp.WithDescription("Append content to a file under FS_ROOT, creating it (and parent dirs) if missing. "+
				"Refuses if resulting file would exceed FS_MAX_FILE_SIZE."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to FS_ROOT")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Content to append")),
		),
		fs.appendFile,
	)
}

func (fs *FilesystemServer) appendFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	full, err := fs.safePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	var existing int64
	if info, err := os.Stat(full); err == nil {
		if info.IsDir() {
			return mcp.NewToolResultError(fmt.Sprintf("%s is a directory", path)), nil
		}
		existing = info.Size()
	}
	if existing+int64(len(content)) > fs.maxSize {
		return mcp.NewToolResultError(fmt.Sprintf("append would exceed max size: %d + %d > %d", existing, len(content), fs.maxSize)), nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mkdir: %v", err)), nil
	}
	f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("open: %v", err)), nil
	}
	defer f.Close()
	if _, err := f.WriteString(content); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("appended %d bytes to %s", len(content), path)), nil
}
