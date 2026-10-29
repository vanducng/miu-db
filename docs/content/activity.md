---
title: Activity log
description: Every query, exec, and schema call is logged as JSONL metadata. Browse, prune, or turn it off.
---

Every `query run`, `schema tree`, and `erd` call from the CLI, and every
`query_run` and `schema_tree` call through the MCP server, writes one JSONL event
to:

```text
~/.config/miu/db/activity/<date>/<session_id>.jsonl
```

An event holds the SQL text, `sql_shape` (literals replaced with `?`), connection,
group, `db_type`, op, latency, `rows_returned`, source (`cli` or `mcp`), and error
info. Result rows are never stored. `query script` and the native `miudb serve`
protocol are not logged.

## Browse

```sh
miudb activity --since 24h -o pretty
miudb activity --connection team/app --since 7d
miudb activity --failed --since 24h
miudb activity --session <session_id>        # one trace, across date directories
```

| Flag | Effect |
|---|---|
| `--since` | Newer than a duration: `1h`, `24h`, `7d`, `2w` |
| `--connection` | Bare name or `group/name` |
| `--group` | One group |
| `--failed` | Only events with errors |
| `--session` | One session id |
| `--limit` | Max events, `0` for no cap |

## Prune

```sh
miudb activity prune --older-than 30d --dry-run
miudb activity prune --older-than 30d
```

## Turn it off

| Scope | How |
|---|---|
| One call | `--no-activity-log` |
| Everywhere | `MIUDB_ACTIVITY_LOG=off` |
| One connection, keep shapes only | `"log_sql": false` on the connection in `connections.json`. `sql_shape` is still recorded; raw SQL is not. |

The activity `session_id` is minted per CLI call or MCP server start. It is
unrelated to `query run --session key=value`, which sets per-call database
context.

## Query it with DuckDB

```sql
select session_id, connection, op, sql_shape, latency_ms, rows_returned, ts
from read_json_auto('~/.config/miu/db/activity/**/*.jsonl')
order by ts desc
limit 100;
```
