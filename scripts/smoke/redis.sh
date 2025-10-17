#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN=$ROOT_DIR/bin/mcp-redis
ADDR=${REDIS_ADDR:-${REDIS_TEST_ADDR:-127.0.0.1:56379}}

if [ ! -x "$BIN" ]; then
    (cd "$ROOT_DIR/servers/redis" && go build -o "$BIN" .)
fi

REQUESTS='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"set","arguments":{"key":"smoke","value":"ok"}}}
{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"get","arguments":{"key":"smoke"}}}'

OUTPUT=$(REDIS_ADDR="$ADDR" REDIS_PREFIX="smoke:" LOG_LEVEL=error \
    /bin/sh -c 'echo "$1" | "$2"' _ "$REQUESTS" "$BIN" 2>/dev/null || true)

if ! echo "$OUTPUT" | grep -q '"name":"set"'; then
    echo "FAIL: set missing from tools/list" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

if ! echo "$OUTPUT" | grep -q '"ok"'; then
    echo "FAIL: get did not return expected value" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

echo "OK: redis smoke passed"
