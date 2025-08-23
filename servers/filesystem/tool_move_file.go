package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemServer) registerMoveFile() {
	fs.mcp.AddTool(
		mcp.NewTool("move_file",
			mcp.WithDescription("Move or rename a file or directory within FS_ROOT. "+
				"Both source and destination must resolve inside the sandbox."),
			mcp.WithString("from", mcp.Required(), mcp.Description("Source path relative to FS_ROOT")),
			mcp.WithString("to", mcp.Required(), mcp.Description("Destination path relative to FS_ROOT")),
		),
		fs.moveFile,
	)
}

func (fs *FilesystemServer) moveFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()
	from, _ := args["from"].(string)
	to, _ := args["to"].(string)

	src, err := fs.safePath(from)
	if err != nil {
		return mcp.NewToolResultError("from: " + err.Error()), nil
	}
	dst, err := fs.safePath(to)
	if err != nil {
		return mcp.NewToolResultError("to: " + err.Error()), nil
	}
	if _, err := os.Stat(src); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stat from: %v", err)), nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("mkdir dst: %v", err)), nil
	}
	if err := os.Rename(src, dst); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rename: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("moved %s → %s", from, to)), nil
}
