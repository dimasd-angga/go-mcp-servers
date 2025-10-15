# Contributing

Thanks for considering a contribution. This document describes how to add a new server, modify an existing one, and what the release gate looks like.

## Layout

- `shared/` — packages imported by every server (logger, auth, testutil). Servers depend on shared; shared never depends on servers.
- `servers/<name>/` — one Go module per server. Standalone binary plus importable package.
- `scripts/smoke/<name>.sh` — stdio-level end-to-end smoke for each server.
- `deploy/docker-compose.yml` — local Postgres (port 55432) and Redis (port 56379) for integration tests.

## Adding a new server

1. Create `servers/<name>/` with `go.mod` pinning `github.com/mark3labs/mcp-go v0.31.0` and the shared module via a local `replace` directive.
2. Add the path to `go.work`.
3. Implement `NewXServer()` returning a struct that exposes `MCP() *server.MCPServer`. Register tools in a `registerTools()` method.
4. Wire `main.go` for stdio + SSE transports.
5. Add unit tests for every tool, plus integration tests gated by an env var.
6. Add a `scripts/smoke/<name>.sh` script that exercises one tool via real stdio JSON-RPC.
7. Add the server to the `SERVERS` list in `Makefile`.
8. Write `servers/<name>/README.md` documenting tools, env vars, and security model.

## The gate

```bash
make verify        # lint + test-all + smoke
```

All three must be green. CI runs the same command.

## Commit style

Conventional commits:

- `feat(<scope>): ...`
- `fix(<scope>): ...`
- `test(<scope>): ...`
- `docs(<scope>): ...`
- `chore(<scope>): ...`

One logical change per commit.

## Code review checklist

Before opening a PR, confirm:

- Every tool handler returns `*mcp.CallToolResult`, never `error` for user-facing failures.
- Every I/O call uses the `ctx` passed to the handler.
- Sandboxed paths / identifiers / hosts are validated before use.
- Output is size-capped where it could grow unbounded.
- Startup logs include the resolved config so operators can see what the server is doing.
- `go vet`, `go test -race`, and `golangci-lint` are clean.
