package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
)

func (fs *FilesystemServer) registerReadFile() {
	fs.mcp.AddTool(
		mcp.NewTool("read_file",
			mcp.WithDescription("Read the complete contents of a text file under FS_ROOT. "+
				"Returns an error if the file exceeds FS_MAX_FILE_SIZE."),
			mcp.WithString("path",
				mcp.Required(),
				mcp.Description("Path relative to FS_ROOT. Leading '/' is treated as root.")),
		),
		fs.readFile,
	)
}

func (fs *FilesystemServer) readFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	path, _ := req.GetArguments()["path"].(string)
	full, err := fs.safePath(path)
	if err != nil {
		return mcp.NewToolResultError(err.Error()), nil
	}
	info, err := os.Stat(full)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("stat: %v", err)), nil
	}
	if info.IsDir() {
		return mcp.NewToolResultError(fmt.Sprintf("%s is a directory", path)), nil
	}
	if info.Size() > fs.maxSize {
		return mcp.NewToolResultError(fmt.Sprintf("file too large: %d bytes (max %d)", info.Size(), fs.maxSize)), nil
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("read: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}
