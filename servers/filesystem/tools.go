package main

// registerTools wires every filesystem tool to the MCP server. Each tool is
// defined in its own file (tool_<name>.go) to keep handlers small and tests
// co-located with the behavior they exercise.
func (fs *FilesystemServer) registerTools() {
	fs.registerReadFile()
	fs.registerWriteFile()
	fs.registerAppendFile()
	fs.registerDeleteFile()
	fs.registerListDirectory()
	fs.registerCreateDirectory()
	fs.registerMoveFile()
	fs.registerSearchFiles()
	fs.registerGetFileInfo()
	fs.registerFindInFiles()
}
