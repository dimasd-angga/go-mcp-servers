#!/usr/bin/env bash
# Smoke test for mcp-homeassistant. Starts a python http.server-style mock HA.
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
BIN=$ROOT_DIR/bin/mcp-homeassistant

if [ ! -x "$BIN" ]; then
    (cd "$ROOT_DIR/servers/homeassistant" && go build -o "$BIN" .)
fi

# Stand up a tiny mock HA via python3.
MOCK_PORT=18123
python3 -c '
import http.server, json, threading

class H(http.server.BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/states":
            payload = json.dumps([
                {"entity_id":"light.kitchen","state":"on","attributes":{}}
            ]).encode()
            self.send_response(200); self.send_header("Content-Type","application/json")
            self.send_header("Content-Length",str(len(payload))); self.end_headers()
            self.wfile.write(payload); return
        self.send_response(404); self.end_headers()
    def log_message(self, *a, **k): pass

http.server.HTTPServer(("127.0.0.1",18123), H).serve_forever()
' &
MOCK_PID=$!
trap "kill $MOCK_PID 2>/dev/null || true" EXIT

# Wait for mock to be ready.
for _ in 1 2 3 4 5; do
    if curl -s -o /dev/null "http://127.0.0.1:${MOCK_PORT}/api/states"; then break; fi
    sleep 0.5
done

REQUESTS='{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1.0.0"}}}
{"jsonrpc":"2.0","method":"notifications/initialized"}
{"jsonrpc":"2.0","id":2,"method":"tools/list"}
{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"get_states","arguments":{}}}'

OUTPUT=$(HA_URL="http://127.0.0.1:${MOCK_PORT}" HA_TOKEN="smoke" LOG_LEVEL=error \
    /bin/sh -c 'echo "$1" | "$2"' _ "$REQUESTS" "$BIN" 2>/dev/null || true)

if ! echo "$OUTPUT" | grep -q '"name":"get_states"'; then
    echo "FAIL: get_states missing from tools/list" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

if ! echo "$OUTPUT" | grep -q "light.kitchen"; then
    echo "FAIL: get_states did not return mock entity" >&2
    echo "$OUTPUT" >&2
    exit 1
fi

echo "OK: homeassistant smoke passed"
