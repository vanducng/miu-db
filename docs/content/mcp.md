---
title: MCP server
description: Configure miu-db as a local stdio MCP server for coding agents.
---

# MCP Server

`miudb mcp serve --transport stdio` exposes saved miu-db connections to MCP
clients over standard input/output. It is separate from the native
`miudb serve --protocol jsonrpc|ndjson` protocol used by Neovim and custom
clients.

## Command

```bash
miudb mcp serve --transport stdio
```

Useful flags:

- `--connection <name>`: repeat to restrict visible and callable connections.
- `--limit <n>`: default row limit for `query_run`.
- `--max-limit <n>`: upper bound accepted from tool arguments.
- `--max-bytes <n>`: maximum serialized tool or resource response size.
- `--timeout <duration>`: inherited global timeout, such as `30s`.
- `--allow-mutate`: permit mutation SQL through `query_run`; unsafe and off by
  default.

:::note[Important]
Stdout is reserved for MCP protocol frames. Diagnostics, startup errors, and
SDK logs go to stderr.
:::

## Safety Model

MCP v1 is local stdio only. There is no remote HTTP transport or remote auth in
this release.

- Connections are loaded from the normal native store under
  `~/.config/miu/db` unless CLI flags override it.
- Connection output is redacted with the same helpers as the CLI.
- Passwords, SSH passwords, private key content, credential file content, and
  credential-bearing URLs are not serialized.
- `query_run` is read-only by default. Mutation statements, uncertain
  multi-statements, mutating CTEs, `EXPLAIN`, and `PRAGMA` are rejected unless
  `--allow-mutate` is set.
- Continuation cursors are signed for the running MCP server and scoped to the
  source connection, so allowlists also apply to `query_fetch_page`.
- Large outputs fail with `query.output_too_large`; lower `--limit`, increase
  `--max-bytes`, or fetch narrower data.

## Tools

- `connections_list`: list redacted MCP-visible connections and store metadata.
- `connection_describe`: return redacted metadata for one connection.
- `connection_test`: open one connection and report reachability.
- `connections_smoke`: run a small read-only smoke query across visible
  connections or a named subset.
- `schema_tree`: inspect schema objects for one connection.
- `query_run`: run bounded read-only SQL and return columns plus row arrays.
- `query_fetch_page`: fetch a continuation cursor returned by `query_run`.

## Resources

- `miudb://connections`
- `miudb://connections/{name}`
- `miudb://connections/{name}/schema`
- `miudb://queries/{cursor}`

Schema resources can be large. Prefer `schema_tree` when an agent needs bounded,
interactive control over what it reads.

## Host Setup

### Codex

Codex reads MCP server entries from `~/.codex/config.toml` under
`mcp_servers.<id>`.

```toml
[mcp_servers.miudb]
command = "miudb"
args = ["mcp", "serve", "--transport", "stdio"]
startup_timeout_sec = 10
tool_timeout_sec = 60
```

To restrict the server to one connection:

```toml
[mcp_servers.miudb]
command = "miudb"
args = ["mcp", "serve", "--transport", "stdio", "--connection", "local-app"]
```

Reference: [OpenAI Codex configuration reference](https://developers.openai.com/codex/config-reference).

### Claude Code

Use the CLI:

```bash
claude mcp add --transport stdio miudb -- miudb mcp serve --transport stdio
```

Or add project-scoped `.mcp.json`:

```json
{
  "mcpServers": {
    "miudb": {
      "type": "stdio",
      "command": "miudb",
      "args": ["mcp", "serve", "--transport", "stdio"],
      "env": {}
    }
  }
}
```

Reference: [Claude Code MCP documentation](https://code.claude.com/docs/en/mcp).

### Cursor

Use `.cursor/mcp.json` for project configuration or `~/.cursor/mcp.json` for
global configuration.

```json
{
  "mcpServers": {
    "miudb": {
      "type": "stdio",
      "command": "miudb",
      "args": ["mcp", "serve", "--transport", "stdio"]
    }
  }
}
```

Reference: [Cursor MCP documentation](https://docs.cursor.com/context/model-context-protocol).

### VS Code and Copilot

Use `.vscode/mcp.json` for workspace configuration or your VS Code user profile
MCP configuration.

```json
{
  "servers": {
    "miudb": {
      "type": "stdio",
      "command": "miudb",
      "args": ["mcp", "serve", "--transport", "stdio"]
    }
  }
}
```

Reference: [VS Code MCP configuration reference](https://code.visualstudio.com/docs/copilot/reference/mcp-configuration).

## Troubleshooting

- If the host cannot start the server, use a full path from `which miudb`.
- If no tools appear, restart the host and verify its MCP panel or command.
- If a connection is hidden, check repeated `--connection` flags.
- If SQL is rejected as read-only, simplify to a single `select` statement.
- If output is too large, lower `--limit` or query fewer columns.
