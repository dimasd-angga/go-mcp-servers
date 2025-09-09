#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN=$ROOT_DIR/bin/mcp-shell

if [ ! -x "$BIN" ]; then
    (cd "$ROOT_DIR/servers/shell" && go build -o "$BIN" .)
fi

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

REQUESTS='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"run_command","arguments":{"command":"echo smoke-shell"}}}'

OUTPUT=$(SHELL_WORKDIR="$WORKDIR" SHELL_ALLOWED_CMDS="echo" LOG_LEVEL=error \
    /bin/sh -c 'echo "$1" | "$2"' _ "$REQUESTS" "$BIN" 2>/dev/null || true)

if ! echo "$OUTPUT" | grep -q '"name":"run_command"'; then
    echo "FAIL: run_command missing from tools/list" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "smoke-shell"; then
    echo "FAIL: run_command did not return expected stdout" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

echo "OK: shell smoke passed"
