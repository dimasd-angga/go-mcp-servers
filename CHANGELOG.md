# Changelog

All notable changes to this project will be documented in this file.
The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Six MCP servers in Go: `filesystem`, `postgres`, `shell`, `redis`, `http`, `homeassistant`.
- Shared packages: `logger` (zerolog → stderr), `auth` (constant-time token check), `testutil` (in-process MCP client helpers).
- `go.work` multi-module workspace targeting Go 1.23.
- `make verify` quality gate: lint + unit tests + stdio smoke tests.
- Docker Compose for local Postgres (port 55432) and Redis (port 56379).
- GitHub Actions CI: build + test + smoke across the workspace, with Postgres and Redis service containers.
- GoReleaser config producing static binaries for darwin/linux × amd64/arm64.
- Per-server READMEs documenting tools, env vars, security model.
