# `mcp-filesystem` — Filesystem MCP server (Go)

Sandboxed filesystem operations for AI agents over the Model Context Protocol. Read, write, search, and manage files inside a configurable root directory. All paths are validated against the sandbox before any I/O.

## Tools

| Tool | Description |
|---|---|
| `read_file` | Read a text file under `FS_ROOT`. Rejects files larger than `FS_MAX_FILE_SIZE`. |
| `write_file` | Write or overwrite a file. Creates parent directories. |
| `append_file` | Append to a file, creating it if missing. |
| `delete_file` | Delete a file. Requires `FS_ALLOW_DELETE=true`. |
| `list_directory` | List entries in a directory; `recursive` walks subtrees. |
| `create_directory` | Make a directory tree under `FS_ROOT`. |
| `move_file` | Rename or move a file inside the sandbox. |
| `search_files` | Glob search by filename or relative path. |
| `get_file_info` | Stat a path; returns JSON with type, size, mtime, mode. |
| `find_in_files` | Grep-like text search across files. |

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `FS_ROOT` | yes | – | Absolute directory the server is sandboxed to. Must exist. |
| `FS_MAX_FILE_SIZE` | no | `10485760` (10 MB) | Maximum bytes for read/write/append operations. |
| `FS_ALLOW_DELETE` | no | `false` | Set to `true` to enable the `delete_file` tool. |
| `LOG_LEVEL` | no | `info` | `debug` for verbose output. |
| `MCP_AUTH_TOKEN` | no | – | If set, requires matching token (transport-level). |

## Run

```bash
# stdio (Claude Desktop, embedded use)
FS_ROOT=$HOME/workspace ./mcp-filesystem

# SSE (network)
FS_ROOT=$HOME/workspace ./mcp-filesystem --transport=sse --port=8001
```

## Claude Desktop config

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/usr/local/bin/mcp-filesystem",
      "env": {
        "FS_ROOT": "/Users/me/workspace",
        "FS_ALLOW_DELETE": "false"
      }
    }
  }
}
```

## Security

- Every path is canonicalized and checked with `filepath.Rel` against `FS_ROOT`. Sibling directories with shared prefixes (`/tmp/root` vs `/tmp/rootevil`) are correctly rejected.
- Absolute paths supplied to a tool are reinterpreted as root-relative; the leading `/` is stripped.
- Symlinks under `FS_ROOT` are followed (`os.Stat` semantics). Be mindful when allowing untrusted symlinks inside the sandbox.
- `delete_file` is off by default and refuses to delete directories even when enabled.
- File reads stat first, then read — oversize files never enter memory.

## Build

```bash
go build -o mcp-filesystem .
# or from workspace root:
make build-all
```
