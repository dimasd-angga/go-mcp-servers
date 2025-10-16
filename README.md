# go-mcp-servers

> Production-ready **Model Context Protocol (MCP)** servers in **Go** — filesystem, PostgreSQL, Redis, shell, HTTP, and Home Assistant. Embed as static binaries or import as Go packages.

[![CI](https://github.com/dimasd-angga/go-mcp-servers/actions/workflows/ci.yml/badge.svg)](https://github.com/dimasd-angga/go-mcp-servers/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go 1.23+](https://img.shields.io/badge/go-1.23%2B-00ADD8?logo=go)](https://go.dev/)
[![MCP Spec](https://img.shields.io/badge/MCP-2024--11--05-purple)](https://modelcontextprotocol.io)

`go-mcp-servers` fills the gap left by the official MCP reference servers — which only ship in **TypeScript** and **Python**. Use these in Claude Desktop, Cursor, custom Go agents, or any MCP-compatible host. Each server is a single static binary (~9 MB) and also importable as a Go package.

---

## Servers

| Server | Description | Docs |
|---|---|---|
| [`filesystem`](servers/filesystem/README.md) | Sandboxed read / write / search / list / move under a configurable root. | [README](servers/filesystem/README.md) |
| [`postgres`](servers/postgres/README.md) | Query, introspect, and optionally mutate PostgreSQL. Tools for schema, indexes, EXPLAIN. | [README](servers/postgres/README.md) |
| [`shell`](servers/shell/README.md) | Sandboxed shell execution with allowlist, denylist, ANSI strip, timeout. | [README](servers/shell/README.md) |
| [`redis`](servers/redis/README.md) | Strings, lists, hashes, pub/sub, TTL — with namespace prefix. | [README](servers/redis/README.md) |
| [`http`](servers/http/README.md) | HTTP client (GET/POST/PUT/DELETE/custom) with allowlist, JSON / HTML parsing. | [README](servers/http/README.md) |
| [`homeassistant`](servers/homeassistant/README.md) | Home Assistant entity control via the HA REST API. | [README](servers/homeassistant/README.md) |

---

## Quick start

### Build from source

```bash
git clone https://github.com/dimasd-angga/go-mcp-servers
cd go-mcp-servers
make build-all
# binaries appear in ./bin/
```

### Run one

```bash
FS_ROOT=$HOME/workspace ./bin/mcp-filesystem
```

### Use in Claude Desktop

Edit `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "filesystem": {
      "command": "/absolute/path/to/bin/mcp-filesystem",
      "env": { "FS_ROOT": "/Users/me/workspace" }
    }
  }
}
```

Restart Claude Desktop. The filesystem tools appear automatically. Ask: *"Read README.md and summarize it."*

---

## Why Go?

The official [`modelcontextprotocol/servers`](https://github.com/modelcontextprotocol/servers) repo ships only TypeScript and Python. For Go-native AI agent runtimes — like Tauri desktop agents, Ollama-adjacent Go services, or anything embedding MCP into a Go process — that's a friction point. `go-mcp-servers` exists so Go projects can:

- **Ship single static binaries** — no Node or Python runtime on the host.
- **Embed in-process** — import the server as a Go package, host the tools without a subprocess.
- **Run with low overhead** — ~9 MB binaries, ~15 MB RSS idle, sub-500ms cold start.
- **Pass `go vet`, race detector, `gosec`** — production-ready guarantees from the toolchain.

If you're on TypeScript or Python, use the official reference servers. If you're on Go, use this.

---

## All six in one config

```json
{
  "mcpServers": {
    "filesystem":    { "command": "/usr/local/bin/mcp-filesystem",    "env": { "FS_ROOT": "/Users/me/workspace" } },
    "postgres":      { "command": "/usr/local/bin/mcp-postgres",      "env": { "POSTGRES_DSN": "postgres://reader:secret@db:5432/app?sslmode=require" } },
    "shell":         { "command": "/usr/local/bin/mcp-shell",         "env": { "SHELL_WORKDIR": "/Users/me/workspace", "SHELL_ALLOWED_CMDS": "go,npm,git,make" } },
    "redis":         { "command": "/usr/local/bin/mcp-redis",         "env": { "REDIS_ADDR": "localhost:6379", "REDIS_PREFIX": "claude:" } },
    "http":          { "command": "/usr/local/bin/mcp-http",          "env": { "HTTP_ALLOWED_HOSTS": "api.github.com,httpbin.org" } },
    "homeassistant": { "command": "/usr/local/bin/mcp-homeassistant", "env": { "HA_URL": "http://homeassistant.local:8123", "HA_TOKEN": "..." } }
  }
}
```

---

## Use from custom Go agents

Each server is also a Go package — import it and host the MCP server in-process.

```go
package main

import (
    "log"

    fsserver "github.com/dimasd-angga/go-mcp-servers/servers/filesystem"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    fs, err := fsserver.NewFilesystemServer()
    if err != nil {
        log.Fatal(err)
    }
    if err := server.ServeStdio(fs.MCP()); err != nil {
        log.Fatal(err)
    }
}
```

---

## Architecture

```
MCP host (Claude Desktop / Cursor / your Go agent)
         │
         ▼  JSON-RPC 2.0  (stdio or SSE)
go-mcp-server binary
         │
         ▼
External resource (filesystem, Postgres, Redis, shell, HTTP, HA)
```

- **Transport:** stdio by default (for Claude Desktop and embedded use) or SSE (`--transport=sse --port=N`).
- **Tools:** each server registers a fixed catalog. No dynamic tool generation.
- **Auth:** optional `MCP_AUTH_TOKEN` enables bearer-token gating across servers.
- **Logging:** structured logs (zerolog) to stderr only. stdout is reserved for the MCP transport.

---

## Comparison with official servers

| Feature | `modelcontextprotocol/servers` | `go-mcp-servers` |
|---|---|---|
| Language | TypeScript, Python | Go |
| Distribution | npm / pip | Single static binary or `go install` |
| Cold start | ~1–3 s | <500 ms |
| Idle RAM | 50–150 MB | ~15 MB |
| Embeddable in Go | ❌ (subprocess only) | ✅ (`import`) |
| Built on | official SDK | [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) |

Pick the one that fits your stack.

---

## Local development

```bash
# Bring up Postgres + Redis on alternate ports (55432, 56379) to avoid host clashes
docker compose -f deploy/docker-compose.yml up -d postgres redis

# Run everything
export POSTGRES_TEST_DSN="postgres://mcptest:mcptest@127.0.0.1:55432/mcptest?sslmode=disable"
export REDIS_TEST_ADDR="127.0.0.1:56379"
export POSTGRES_DSN="$POSTGRES_TEST_DSN"
export REDIS_ADDR="$REDIS_TEST_ADDR"

make test-all      # unit + integration tests
make build-all     # build into ./bin/
make smoke         # exercise each binary via real stdio JSON-RPC
make verify        # lint + test-all + smoke (release gate)
```

CI runs the same `make verify` on every push.

---

## Roadmap

- `mongodb` server
- `slack` server (Slack Web API)
- `github` server (REST + GraphQL)
- Resources and prompts (currently tools-only)
- Streamable HTTP transport once the spec stabilizes
- Pre-built Docker images on `ghcr.io`
- Homebrew formula

Vote in [Discussions](https://github.com/dimasd-angga/go-mcp-servers/discussions).

---

## License

MIT — see [LICENSE](LICENSE).

## Acknowledgments

- [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) — the MCP protocol library underneath every server here.
- [Anthropic](https://anthropic.com) — for the Model Context Protocol specification.
- [`modelcontextprotocol/servers`](https://github.com/modelcontextprotocol/servers) — the TS/Python reference implementations these complement.
