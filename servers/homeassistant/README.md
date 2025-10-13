# `mcp-homeassistant` — Home Assistant MCP server (Go)

Read and control Home Assistant entities via the HA REST API. Designed for personal use and integration into agent runtimes like Gaia.

## Tools

| Tool | Description |
|---|---|
| `get_states` | All entity states; optional `domain` filter (e.g. `light`, `switch`). |
| `get_state` | One entity by `entity_id`. |
| `get_attributes` | Just the attributes object of an entity. |
| `call_service` | Invoke any HA service (e.g. `light.turn_on`) with optional extra data. |
| `get_history` | Recent state history for an entity (last N hours, max 168). |
| `list_automations` | Automations with their on/off state. |
| `toggle_automation` | Enable or disable an automation. |
| `fire_event` | Fire a HA event with optional JSON data. |

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `HA_URL` | yes | – | HA base URL, e.g. `http://homeassistant.local:8123`. Trailing `/` stripped. |
| `HA_TOKEN` | yes | – | Long-lived access token from HA → profile → Security. |
| `HA_TIMEOUT` | no | `10` | Per-request timeout in seconds. |
| `LOG_LEVEL` | no | `info` | `debug` for verbose. |

## Claude Desktop config

```json
{
  "mcpServers": {
    "homeassistant": {
      "command": "/usr/local/bin/mcp-homeassistant",
      "env": {
        "HA_URL": "http://homeassistant.local:8123",
        "HA_TOKEN": "eyJ0..."
      }
    }
  }
}
```

## Security

- Every request sends the bearer token in `Authorization`; tokens never leave the server (no echo, no logging).
- `get_history` caps the window at one week to keep responses bounded.
- The token is read from the environment at startup and not exposed via any tool.
- The server does not implement any user-confirmation step for destructive services like `light.turn_off`. If you wire this into an autonomous agent, control which `domain`/`service` combinations are exposed at the agent layer.
