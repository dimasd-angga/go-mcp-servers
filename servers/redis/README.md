# `mcp-redis` — Redis MCP server (Go)

Strings, lists, hashes, pub/sub, and TTL operations on Redis through the Model Context Protocol. All keys are silently scoped to a configurable prefix so multi-tenant deployments don't collide.

## Tools

| Tool | Description |
|---|---|
| `get` / `set` / `delete` | String key/value with optional `ex` expiry on `set`. |
| `list_keys` | SCAN-based key listing matching a pattern. Returns keys with the prefix stripped. |
| `get_type` | TYPE of a key. |
| `lpush` / `lpop` / `lrange` | List push/pop and range queries. |
| `hset` / `hget` / `hgetall` | Hash field ops. |
| `expire` / `ttl` | TTL management. |
| `publish` | Publish a message to a channel. |

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `REDIS_ADDR` | yes | – | `host:port`. |
| `REDIS_PASSWORD` | no | – | If set, used for AUTH. |
| `REDIS_DB` | no | `0` | Logical DB index. |
| `REDIS_PREFIX` | no | – | Prepended to every key sent to Redis and stripped from output. Use for tenant isolation. |
| `LOG_LEVEL` | no | `info` | `debug` for verbose. |

## Run

```bash
REDIS_ADDR=localhost:6379 REDIS_PREFIX="myapp:" ./mcp-redis
```

## Claude Desktop config

```json
{
  "mcpServers": {
    "redis": {
      "command": "/usr/local/bin/mcp-redis",
      "env": {
        "REDIS_ADDR": "localhost:6379",
        "REDIS_PREFIX": "claude:"
      }
    }
  }
}
```

## Security

- The `REDIS_PREFIX` is applied server-side; agents cannot escape their namespace by crafting their own keys.
- `list_keys` uses `SCAN` (not `KEYS`) to avoid blocking the Redis server on large keyspaces.
- Tools that return "missing" data (e.g. `get` on a non-existent key) return a structured error result so agents can react explicitly.

## Integration test setup

```bash
docker compose -f deploy/docker-compose.yml up -d redis
export REDIS_TEST_ADDR="127.0.0.1:56379"
go test ./...
```
