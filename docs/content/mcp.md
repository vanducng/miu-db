---
title: MCP server
description: Run miudb as a local stdio MCP server with read-only, bounded database tools for Claude Code, Codex, Cursor, and VS Code.
---

```sh
miudb mcp serve --transport stdio
```

The server reads the normal connection store and exposes it over stdio only;
there is no HTTP transport. Stdout carries MCP frames only; diagnostics and SDK
logs go to stderr. Unknown transports exit 2.

| Flag | Default | Effect |
|---|---|---|
| `--connection <ref>` | all | Repeat to restrict which connections are visible and callable |
| `--limit` | 100 | Default row limit for query tools |
| `--max-limit` | 1000 | Highest limit a tool call may request |
| `--max-bytes` | 1048576 | Max serialized bytes per tool or resource response |
| `--timeout` | 30s | Global connection and query timeout |
| `--allow-mutate` | off | Permit mutation SQL. Unsafe. |

## Add to a host

| Host | Setup |
|---|---|
| Claude Code | `claude mcp add --transport stdio miudb -- miudb mcp serve --transport stdio` |
| Codex | `~/.codex/config.toml`, below |
| Cursor | `.cursor/mcp.json` (project) or `~/.cursor/mcp.json` (global) |
| VS Code, Copilot | `.vscode/mcp.json` or your user MCP config |

```toml
# ~/.codex/config.toml
[mcp_servers.miudb]
command = "miudb"
args = ["mcp", "serve", "--transport", "stdio"]
startup_timeout_sec = 10
tool_timeout_sec = 60
```

Cursor and a project `.mcp.json` for Claude Code use `mcpServers`; VS Code uses
`servers` with the same entry:

```json
{
  "mcpServers": {
    "miudb": { "type": "stdio", "command": "miudb", "args": ["mcp", "serve", "--transport", "stdio"] }
  }
}
```

To pin one connection, append `"--connection", "app"` to `args`. If the host
cannot find `miudb`, use the full path from `which miudb`.

## Tools

| Tool | Does |
|---|---|
| `connections_list` | Redacted connections visible to this server |
| `connection_describe` | Redacted metadata for one connection |
| `connection_test` | Open one connection; may start a tunnel, runs no user SQL |
| `connections_smoke` | Bounded smoke query across visible connections or a subset |
| `schema_tree` | Tables, views, and columns for one connection |
| `query_run` | One read-only statement; columns plus row arrays and a cursor |
| `query_script` | Multi-statement script, Snowflake and MySQL only; `atomic` wraps it in a transaction |
| `query_fetch_page` | Next page for a `query_run` cursor |

Resources: `miudb://connections`, `miudb://connections/{name}`,
`miudb://connections/{name}/schema`, `miudb://queries/{cursor}`. Schema
resources can be large; prefer `schema_tree`.

## Safety

- `query_run` rejects mutations, uncertain multi-statements, mutating CTEs,
  `EXPLAIN`, and `PRAGMA` with `query.read_only_violation` unless the server
  runs with `--allow-mutate`.
- A mutating `query_script` needs `--allow-mutate` on the server and
  `allow_mutate=true` on the call (`query.mutation_not_acknowledged` otherwise).
  Read-only scripts always run.
- Connection output is redacted with the CLI helpers. Passwords, SSH passwords,
  private keys, credential file contents, and credential-bearing URLs are never
  serialized.
- Cursors are signed for the running server and scoped to their connection, so
  the `--connection` allowlist also applies to `query_fetch_page`.
- An oversized response fails with `query.output_too_large`: lower the limit,
  select fewer columns, or raise `--max-bytes`.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Host cannot start the server | Use the full path from `which miudb` |
| No tools appear | Restart the host; check its MCP panel |
| A connection is missing | Check the repeated `--connection` flags |
| `query.read_only_violation` | Use a single `select`, or restart with `--allow-mutate` if you mean it |
