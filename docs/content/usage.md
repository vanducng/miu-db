---
title: Connections and queries
description: Add, store, share, and test connections, then run bounded queries, scripts, and schema inspection.
---

## Where config lives

| Path | Holds |
|---|---|
| `~/.config/miu/db/connections.json` | Connection metadata. No secrets unless `--secret-store inline`. |
| `~/.config/miu/db/credentials.json` | `file` secret store, mode `0600`. Falls back to `credentials-export.json` in the same directory when absent. |
| `~/.config/miu/db/keyring/` | Encrypted file keyring used by release builds (see below). |
| `~/.config/miu/db/activity/` | [Activity log](/activity/). |

Override the directory with `MIUDB_CONFIG_DIR` or `--config-dir`. Other global
store flags: `--connections-file`, `--credentials-file`, `--secret-source`
(`file,keyring,gopass,none`), `--keyring-service` (default `miudb`),
`--gopass-prefix` (default `miudb`). Only pass them for a non-default store.

## Add a connection

```sh
miudb connections add --name app --db-type sqlite --path ./app.db

miudb connections add --name app-dev --db-type postgresql \
  --host localhost --port 5432 --database app --username app \
  --password "$APP_DB_PASSWORD"

miudb connections add --name app-prod --group team --db-type mysql \
  --host db.internal.example --port 3306 --database app --username app \
  --password "$APP_DB_PASSWORD" --tunnel --ssh-config-alias bastion
```

| Flag | Notes |
|---|---|
| `--db-type` | `sqlite`, `postgresql`, `mysql`, `snowflake`, `bigquery` |
| `--group` | Nested paths allowed (`team/project`). `--folder` is a deprecated alias. |
| `--url` | Connection URL instead of host/port/database |
| `--password-command` | Command that prints the password at connect time. Use only commands you trust. |
| `--option k=v` | Provider settings, repeatable. Snowflake and BigQuery keys: [Authentication](/authentication/). |
| `--extra-option k=v` | Driver extra options, for example `sslmode=require` |
| `--tunnel` | SSH tunnel. Use `--ssh-config-alias`, or `--ssh-host`, `--ssh-port` (default 22), `--ssh-username`, `--ssh-key-path`, `--ssh-password`. |
| `--secret-store` | Where passwords go: `keyring` (default), `file`, `inline`, `none` |

Full list: `miudb connections add --help`.

### Secret stores

| Store | Behavior |
|---|---|
| `keyring` | OS keychain service `miudb` on cgo builds. Release binaries are built without cgo and use an encrypted file keyring at `~/.config/miu/db/keyring/<connection>:<kind>` instead, so the macOS Keychain shows nothing. |
| `file` | `credentials.json`, mode `0600` |
| `inline` | Kept in `connections.json` |
| `none` | Discarded; another resolver must supply it |

At query time secrets resolve from the file store, the keyring, and `gopass`
paths under the `miudb` prefix. `connections list` shows `has_password: true`,
never the value; `connections test` proves the secret resolves.

## Address a connection

`miudb connections list --basic` prints `ref`, `name`, `group`, `db_type`, and
`host`. Pass the `ref` to `--connection`: `group/name`, or the bare `name` when it
is unique. `-c` is a short form only on `erd` commands.

## Share and import

```sh
miudb connections import ./shared.json --dry-run   # preview added and overwritten
miudb connections import ./shared.json
```

Import merges by `name`: a matching name is overwritten, and the existing
`connections.json` is first copied to `connections.json.bak-<timestamp>`. Secrets
import as they appear in the file; a file with inline passwords is plaintext, so
keep it `0600` and share it over a secure channel.

## Test

```sh
miudb connections test app-dev                 # open one connection, no user SQL
miudb connections smoke --timeout 20s          # every connection, one envelope
miudb connections smoke --connection a --connection b --sql 'select 1'
```

`smoke` runs `select 1 as one` with `--concurrency 4` by default. It exits 0 even
when some connections fail; read `data.results[].ok` and `summary.failed`. Local
connections fail when their database is stopped; compare tunnel failures against
a known-good client before calling it a miudb bug.

## Query

```sh
miudb query run --connection app --sql 'select * from orders' --limit 100
miudb query fetch-page --cursor <page.next_cursor>
```

`query run` executes one statement. `--limit` (global, default 100) bounds the
inline rows; when more exist the result has `truncated: true` and
`page.next_cursor`. `--timeout` (global, default 30s) bounds the connection and
query. `query run` does not block mutations; the read-only guard is in
[MCP](/mcp/).

### Scripts

```sh
miudb query script --connection wh --sql "show users like 'alice'; show roles"
```

Snowflake and MySQL only. Returns `data.results`, one entry per statement with
`kind` `rows` or `exec`. Fails fast; `--atomic` wraps the script in a transaction
(DML-only atomicity). SQLite, Postgres, and BigQuery return
`query.script_unsupported`.

### Per-call session

`--session key=value` (repeatable, on `query run` and `query script`) sets context
for one call only. Unsupported keys fail with `query.unsupported_session_key`.

| Provider | Keys |
|---|---|
| Snowflake | `role`, `warehouse`, `database`, `schema` |
| Postgres | `role`, `search_path`, `statement_timeout`, `application_name` |
| MySQL | `sql_mode`, `time_zone`, `max_execution_time` |
| BigQuery | `bigquery_location`, `bigquery_maximum_bytes_billed` |

## Schema

```sh
miudb schema tree --connection app
```

Returns tables, views, and columns. Metadata SQL through `query run` also works:
`information_schema` on Postgres and Snowflake, `SHOW TABLES` / `DESCRIBE` on
MySQL, `INFORMATION_SCHEMA` on BigQuery, `sqlite_master` and `PRAGMA table_info`
on SQLite. For a diagram, see [ERD](/erd/).
