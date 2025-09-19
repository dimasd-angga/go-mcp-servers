# `mcp-postgres` — PostgreSQL MCP server (Go)

Query, introspect, and (optionally) mutate a PostgreSQL database through the Model Context Protocol. Schema-aware tools let an AI agent explore an unfamiliar database without ever seeing the connection string.

## Tools

| Tool | Description |
|---|---|
| `query_rows` | Run a SELECT/WITH query and return rows as JSON. Auto-limited to `PG_MAX_ROWS`. |
| `execute` | Run INSERT/UPDATE/DELETE/DDL. Requires `PG_ALLOW_WRITE=true`. |
| `list_tables` | List base tables in a schema (default `public`). |
| `describe_table` | Column names, types, nullability, defaults. |
| `list_schemas` | User schemas (excludes `pg_*`, `information_schema`). |
| `get_table_indexes` | Index name + DDL for a table. |
| `count_rows` | `SELECT COUNT(*)` with optional WHERE. |
| `explain_query` | `EXPLAIN (FORMAT JSON)` — never `ANALYZE`. |

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `POSTGRES_DSN` | yes | – | `postgres://user:pass@host:port/db?sslmode=disable` style. |
| `PG_ALLOW_WRITE` | no | `false` | Set to `true` to allow `execute` and non-SELECT statements in `query_rows`. |
| `PG_MAX_ROWS` | no | `500` | Auto-appended `LIMIT` for SELECT queries. |
| `PG_QUERY_TIMEOUT` | no | `30` | Per-query timeout in seconds. |
| `LOG_LEVEL` | no | `info` | `debug` for verbose. |

## Run

```bash
POSTGRES_DSN="postgres://user:pass@localhost:5432/db?sslmode=disable" ./mcp-postgres
```

## Claude Desktop config

```json
{
  "mcpServers": {
    "postgres": {
      "command": "/usr/local/bin/mcp-postgres",
      "env": {
        "POSTGRES_DSN": "postgres://reader:secret@db:5432/app?sslmode=require",
        "PG_ALLOW_WRITE": "false"
      }
    }
  }
}
```

## Security

- Connection pool capped at 5 open / 2 idle to prevent runaway connections.
- `query_rows` rejects anything that doesn't start with `SELECT` or `WITH` unless `PG_ALLOW_WRITE=true`.
- `count_rows` validates the table identifier with a strict regex; injected suffixes such as `users; DROP TABLE x` are rejected before SQL is built.
- `LIMIT` is appended for SELECT/WITH queries that don't already contain one.
- `EXPLAIN_QUERY` never adds `ANALYZE` — it cannot mutate.
- Every query runs inside a context with `PG_QUERY_TIMEOUT` so runaway queries get cancelled server-side.

## Integration test setup

The bundled `deploy/docker-compose.yml` runs Postgres on host port `55432` (the non-default port avoids clashes with a host Postgres). Run tests with:

```bash
docker compose -f deploy/docker-compose.yml up -d postgres
export POSTGRES_TEST_DSN="postgres://mcptest:mcptest@127.0.0.1:55432/mcptest?sslmode=disable"
go test ./...
```
