---
title: Architecture
description: How the CLI, MCP server, and stdio protocol share one Go core, plus package boundaries, stores, and the tech stack.
---

miudb is one pure-Go static binary (`CGO_ENABLED=0` in releases). Three entry
points share one service layer, `core.Services`, which resolves a connection,
opens it through an adapter, bounds the result, and logs the call.

<a href="/diagrams/architecture.html"><img class="dark:sl-hidden" src="/diagrams/architecture-light.png" alt="miu-db architecture: the miudb CLI, miudb mcp serve, and miudb serve all call core.Services. core.Services resolves the connection ref and secrets from the config store, opens and queries through the adapter registry, saves and fetches extra rows in the page store, and writes events to the activity log. Adapters reach SQLite, Postgres, MySQL, Snowflake, and BigQuery directly, or through an SSH tunnel for Postgres and MySQL when one is enabled." width="1600" height="636" />
<img class="light:sl-hidden" src="/diagrams/architecture-dark.png" alt="" width="1600" height="636" /></a>

Click the diagram for the interactive version.

## Packages

| Layer | Package | Job |
|---|---|---|
| Entry | `cmd/miudb`, `internal/cli` | Cobra commands, flags, the `miudb.cli/v1` envelope, exit codes, `upgrade` |
| Entry | `internal/mcpserver` | MCP stdio tools and resources, read-only guard, allowlist, byte caps, signed cursors |
| Entry | `internal/protocol` | `miudb serve`: JSON-RPC or NDJSON over stdio with call events |
| Core | `internal/core` | `Services`: the one seam the three entry points call |
| Config | `internal/config` | Connection store, `SecretRef` resolvers (keyring, file, gopass, command, env), OAuth token store, redaction |
| Auth | `internal/auth` | Snowflake OAuth loopback flow with PKCE |
| Query | `internal/query`, `internal/result` | Run, script, and page; leftover rows saved under `$TMPDIR/miudb-pages` |
| Adapters | `internal/adapter`, `internal/adapters/*` | Provider interface and registry; SQLite, Postgres, MySQL, Snowflake, BigQuery |
| Tunnel | `internal/tunnel` | SSH local forwards for Postgres and MySQL, `~/.ssh/config` aliases |
| Schema | `internal/schema`, `internal/erd` | Schema tree; ERD introspection (SQLite, Postgres, MySQL), HTML, DBML |
| Activity | `internal/activity` | JSONL event writer and reader |
| Client | `ui/miu-db.nvim` | Neovim client over `miudb serve`; SQL stays in normal buffers |

Rules that keep the layers honest:

- Entry points hold no database logic; they call `core.Services`.
- Frontends, including the Neovim plugin, never import adapter internals.
- Raw credentials are never logged or serialized. Errors and envelopes pass
  through the redaction helpers in `internal/config` and the output scrubber in
  `internal/cli`.
- New connections store secrets through `SecretRef` providers, not inline.

## Request path

1. The entry point parses input and mints a session id for the activity log.
2. `Store.FindResolved` matches the ref (`group/name` or a unique `name`) and
   resolves secrets, refreshing a Snowflake OAuth token when it is within 5
   minutes of expiry.
3. The registry picks the adapter by `db_type`. Postgres and MySQL open an SSH
   forward first when the connection has a tunnel.
4. The query service runs the SQL, keeps the first `--limit` rows, saves the
   rest to the page store, and returns `page.next_cursor`. Per-call `--session`
   keys are overlaid on a copy of the connection options, never saved.
5. The service writes one activity event, and the entry point renders the
   envelope or MCP result.

## Tech stack

| Area | Choice |
|---|---|
| Language | Go 1.25, module `github.com/vanducng/miu-db` |
| CLI | Cobra |
| MCP | `modelcontextprotocol/go-sdk`, stdio transport |
| Drivers | `modernc.org/sqlite`, `jackc/pgx/v5`, `go-sql-driver/mysql`, `snowflakedb/gosnowflake`, `cloud.google.com/go/bigquery` |
| Secrets | `99designs/keyring` (encrypted file backend without cgo), optional `gopass` CLI |
| Tunnels | `golang.org/x/crypto/ssh` |
| Tests | Table tests, JSON Schema contract checks (`santhosh-tekuri/jsonschema`), env-gated live-DB tests |
| Release | release-please, GoReleaser, Homebrew tap `vanducng/tap` |
| Docs | Astro Starlight on GitHub Pages |
