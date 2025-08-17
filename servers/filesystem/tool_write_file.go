package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemServer) registerWriteFile() {
	fs.mcp.AddTool(
		mcp.NewTool("write_file",
			mcp.WithDescription("Write content to a file under FS_ROOT, creating parent directories as needed. "+
				"Overwrites if the file exists. Refuses content larger than FS_MAX_FILE_SIZE."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Path relative to FS_ROOT")),
			mcp.WithString("content", mcp.Required(), mcp.Description("Full file contents to write")),
		),
		fs.writeFile,
	)
}

func (fs *FilesystemServer) writeFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	path, _ := args["path"].(string)
	content, _ := args["content"].(string)
	full, err := fs.safePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	if int64(len(content)) > fs.maxSize {
		return mcp.NewToolResultError(fmt.Sprintf("content too large: %d bytes (max %d)", len(content), fs.maxSize)), nil
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mkdir: %v", err)), nil
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("write: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("wrote %d bytes to %s", len(content), path)), nil
}
