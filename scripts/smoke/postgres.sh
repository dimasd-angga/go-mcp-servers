#!/usr/bin/env bash
# Smoke test for mcp-postgres. Requires POSTGRES_TEST_DSN to point at a
# reachable database.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN=$ROOT_DIR/bin/mcp-postgres

DSN=${POSTGRES_DSN:-${POSTGRES_TEST_DSN:-postgres://mcptest:mcptest@127.0.0.1:55432/mcptest?sslmode=disable}}

if [ ! -x "$BIN" ]; then
    (cd "$ROOT_DIR/servers/postgres" && go build -o "$BIN" .)
fi

REQUESTS='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"query_rows","arguments":{"sql":"SELECT 1 AS one"}}}'

OUTPUT=$(POSTGRES_DSN="$DSN" LOG_LEVEL=error \
    /bin/sh -c 'echo "$1" | "$2"' _ "$REQUESTS" "$BIN" 2>/dev/null || true)

if ! echo "$OUTPUT" | grep -q '"name":"query_rows"'; then
    echo "FAIL: query_rows missing from tools/list" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

if ! echo "$OUTPUT" | grep -q 'one'; then
    echo "FAIL: query_rows did not return expected column" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

echo "OK: postgres smoke passed"
