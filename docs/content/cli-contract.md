---
title: Agent CLI contract
description: The miudb.cli/v1 JSON envelope, exit codes, error codes, redaction, and command discovery for agents.
---

Every command prints one JSON envelope on stdout. Stderr is diagnostics only.
`-o pretty` indents the same envelope for humans; agents keep the default
compact `json`.

## Envelope

```json
{
  "ok": true,
  "api_version": "miudb.cli/v1",
  "kind": "query.result",
  "command": "query run",
  "request_id": "req_...",
  "summary": { "connection": "app", "db_type": "sqlite" },
  "data": {
    "connection": "app",
    "result": {
      "columns": [{ "name": "id", "type": "INTEGER", "nullable": true }],
      "rows": [[1], [2]],
      "truncated": true
    }
  },
  "page": { "limit": 2, "next_cursor": "..." },
  "artifacts": [],
  "warnings": []
}
```

- `summary`, `data`, `page`, and `stats` are omitted when empty. `artifacts` and
  `warnings` are always arrays.
- Rows are positional arrays matching `columns`. `truncated: true` with
  `page.next_cursor` means `query fetch-page --cursor` has more.
- `erd` commands list written files in `artifacts`.

On failure `ok` is `false`, `kind` is `error`, and there is no `data`:

```json
{ "ok": false, "kind": "error", "command": "query run",
  "error": { "code": "connection.not_found", "message": "connection not found", "retryable": false } }
```

`error` may also carry `hint`, `safe_to_retry`, and `details`.

## Exit codes

| Exit | Meaning |
|---|---|
| `0` | Success. Also `connections smoke` with failed connections: read `data.results[].ok` and `summary.failed`. |
| `2` | Bad input or a known failure: `connection.not_found`, invalid flags or options, unsupported script or session key, ERD errors |
| `1` | Everything else, including driver errors (`internal.error`), unknown flags, and failed upgrades |

Always branch on `ok`, not only the exit code.

## Common error codes

| Code | Meaning |
|---|---|
| `connection.not_found` | No connection with that ref. List them with `connections list --basic`. |
| `connection.missing_input`, `connection.invalid_option` | `connections add` input is incomplete or malformed |
| `connection.import_unreadable`, `connection.import_empty` | The import file cannot be read or has no connections |
| `query.missing_input`, `query.missing_cursor` | `--sql` or `--cursor` missing |
| `query.multi_statement_not_allowed` | Several statements sent to `query run`; use `query script` |
| `query.script_unsupported`, `query.script_failed` | Script on an unsupported backend, or a statement failed |
| `query.unsupported_session_key` | `--session` key the provider does not accept |
| `erd.unsupported_db`, `erd.missing_connection`, `erd.meta_invalid` | ERD input errors |
| `activity.invalid_since`, `activity.invalid_older_than` | Bad duration |
| `upgrade.managed_install` | Installed by Homebrew; use `brew upgrade miudb` |
| `internal.error` | Driver or unexpected error; the message says what failed |

## Redaction

Credential-named fields (`password`, `secret`, `token`, `private_key`,
`api_key`, and similar), password-bearing URLs, and `key=secret` assignments are
replaced before anything reaches stdout. `connections list` shows
`has_password: true`, never the value. Query values are not masked; they are the
user's data.

## Discovery

```sh
miudb commands                 # catalog: name, summary, stability, mutates, side effects
miudb describe query run       # one command
miudb connections list --basic # refs to pass to --connection
```

## Other transports

| Command | For |
|---|---|
| `miudb mcp serve --transport stdio` | MCP hosts. See [MCP server](/mcp/). |
| `miudb serve --protocol jsonrpc` | JSON-RPC 2.0 over stdio with call-event notifications (Neovim, custom clients) |
| `miudb serve --protocol ndjson` | One line-delimited envelope per request, response, and event |

Neither `serve` mode writes human progress to stdout.
