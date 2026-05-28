# Go Track Architecture Notes

Date: 2026-05-28
Branch: `golang`

## Direction

Build the Go version as a headless database core with protocol clients:

```text
Neovim file buffers    CLI / agents       Future TUI
       |                    |                |
       +---------- protocol client ----------+
                            |
                   JSON-RPC / NDJSON stdio
                            |
                        Go core
                            |
        native store, secrets, tunnels, adapters, workers
                            |
                    database drivers
```

The Python implementation remains the reference daily driver until the Go core
is useful enough for real daily databases.

## Product Requirements

- Neovim integration is first-class and uses normal Neovim files/buffers for
  query editing.
- The first transport is JSON-RPC or NDJSON over stdio, so Neovim, shell tools,
  and agents can all drive the same core.
- CLI commands must expose enough structured information for agents to consume:
  JSON output, stable exit codes, machine-readable errors, schema inspection,
  connection inspection, query execution, export, and cancellation where
  applicable.
- Connection support must include SSH tunnels and related tunnel configuration,
  matching the current `miu-db` direction rather than treating tunnels as a
  later UI feature.
- The core owns adapters, connection/session lifecycle, secrets, tunnel setup,
  query calls, workers, schema inspection, result paging, history, and export.
- Frontends are clients. They should not import adapter internals directly.
- Native config lives under `~/.config/miudb` by default. Sensitive fields are
  classified before persistence and resolved through explicit `SecretRef`
  providers such as keyring, file, gopass, command, or env. New native
  connections default to the `miudb` OS Keychain/keyring service.

## Pre-Plan Research Gate

Before writing the implementation plan, incorporate the agent CLI research in
`research-agent-cli-output-formats-20260528.md`.

Planning must explicitly include:

- Stable JSON success and error envelopes for one-shot CLI commands.
- JSON Lines/NDJSON event objects for streaming query, worker, schema, and
  protocol events.
- A `schema`, `describe`, or `commands` command that emits agent-readable
  command metadata, input schemas, output schemas, examples, stability, side
  effects, and error codes.
- Output controls for agents: `--output`, `--fields`, `--limit`, `--cursor`,
  `--compact`, and artifact/file outputs for large results.
- stdout/stderr separation: data on stdout, diagnostics and progress on stderr.
- Bounded default row/token output for query results, with truncation metadata
  and continuation cursors.
- SQL result shape rules: column metadata plus compact row arrays by default,
  with optional object rows for readability.
- Tests that validate representative CLI outputs against JSON Schema.

## Daily Driver Adapter Targets

Initial useful database set:

- SQLite
- PostgreSQL
- MySQL
- Snowflake
- BigQuery

Suggested MVP order:

1. SQLite, to prove local file connections, query execution, schema, and result
   paging with minimal setup.
2. PostgreSQL and MySQL, to prove TCP sessions, common SQL, credentials, and
   cancellation behavior.
3. Snowflake and BigQuery, to prove cloud-driver configuration, auth variants,
   and slower/remote query workflows.

## Borrowed References

- `nvim-dbee`: borrow the Go backend / Lua frontend separation and asynchronous
  call model.
- `usql`: borrow lessons around broad Go database driver support, URL handling,
  build tags, and CLI ergonomics.
- Current Python `miu-db`: use provider, adapter, process worker, explorer,
  query, result, secret, and SSH tunnel behavior as the product reference.

## Non-Goals For The First Spike

- Full Go TUI.
- Full Python adapter parity.
- Legacy config compatibility.
- Heavy autocomplete or SQL parser work.
- Cloud discovery automation.

## First Spike Definition

The spike is successful when this works in the `golang` worktree:

- `miu-db serve` starts a JSON-RPC/NDJSON stdio server.
- SQLite connection config can be created and tested.
- `miu-db query --connection local --sql "select 1" --format json` returns
  structured output for agents.
- A minimal Neovim client under `ui/nvim` can send SQL from a normal buffer and render results
  into a result buffer.
- Query calls have IDs and can emit started, rows/page, done, and error events.
- The protocol shape already leaves room for cancellation and SSH tunnel-backed
  connections.
