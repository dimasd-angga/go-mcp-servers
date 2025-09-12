# `mcp-shell` — Sandboxed shell execution MCP server (Go)

Run shell commands and scripts inside a fixed working directory with hard timeouts, output truncation, an explicit denylist of catastrophic patterns, and an optional command allowlist.

## Tools

| Tool | Description |
|---|---|
| `run_command` | Execute a string via `bash -c`. Returns JSON `{stdout, stderr, exit_code, timed_out}`. |
| `run_script` | Write a script body to a temp file under workdir and run it with the chosen interpreter. |
| `get_env` | Read an environment variable, restricted to names in `SHELL_ENV_PASSTHROUGH`. |

## Environment

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `SHELL_WORKDIR` | yes | – | Directory the server `cd`s into for every command. Must exist. |
| `SHELL_ALLOWED_CMDS` | no | – (allow all) | Comma-separated command prefixes. If set, commands must start with one of them. |
| `SHELL_TIMEOUT` | no | `60` | Per-command timeout in seconds. |
| `SHELL_MAX_OUTPUT` | no | `51200` | Max bytes captured per stream. Excess is replaced with `[output truncated at N bytes]`. |
| `SHELL_ENV_PASSTHROUGH` | no | `PATH` | Comma-separated env names allowed into the child process. |
| `LOG_LEVEL` | no | `info` | `debug` for verbose logging. |

## Denylist

The server refuses any command containing any of these patterns, regardless of allowlist:

- `rm -rf /`, `rm -rf /*`
- `:(){ ` (fork bomb)
- `> /dev/sd`, `> /dev/nvme`
- `mkfs`
- `dd if=/dev/zero of=/dev/`
- `shutdown `, `reboot`

The denylist is intentionally short and conservative; the allowlist is the primary defense.

## Run

```bash
SHELL_WORKDIR=$HOME/projects SHELL_ALLOWED_CMDS="go,make,git" ./mcp-shell
```

## Claude Desktop config

```json
{
  "mcpServers": {
    "shell": {
      "command": "/usr/local/bin/mcp-shell",
      "env": {
        "SHELL_WORKDIR": "/Users/me/projects",
        "SHELL_ALLOWED_CMDS": "go,npm,git,make,cargo",
        "SHELL_TIMEOUT": "120"
      }
    }
  }
}
```

## Security model

- The child process inherits only the variables named in `SHELL_ENV_PASSTHROUGH`. Secrets in the server's environment do not leak.
- Output streams are independently captured and truncated; a runaway logger cannot exhaust memory.
- ANSI escape codes are stripped from output for readability and to defang terminal-control injection.
- Timeouts use `exec.CommandContext`, so the process is killed when the deadline passes; exit code is reported as 124.
