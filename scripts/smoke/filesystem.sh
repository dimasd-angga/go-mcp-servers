#!/usr/bin/env bash
# Smoke test: exercise the filesystem binary over real stdio JSON-RPC.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN=$ROOT_DIR/bin/mcp-filesystem

if [ ! -x "$BIN" ]; then
    echo "building $BIN" >&2
    (cd "$ROOT_DIR/servers/filesystem" && go build -o "$BIN" .)
fi

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

echo "hello smoke" > "$WORKDIR/hello.txt"

REQUESTS='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"read_file","arguments":{"path":"hello.txt"}}}'

OUTPUT=$(FS_ROOT="$WORKDIR" LOG_LEVEL=error \
    /bin/sh -c 'echo "$1" | "$2"' _ "$REQUESTS" "$BIN" 2>/dev/null || true)

if ! echo "$OUTPUT" | grep -q '"name":"read_file"'; then
    echo "FAIL: read_file missing from tools/list" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "hello smoke"; then
    echo "FAIL: read_file did not return expected content" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

echo "OK: filesystem smoke passed"
