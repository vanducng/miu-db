---
title: ERD diagrams
description: Turn a SQLite, Postgres, or MySQL connection into an offline interactive ER diagram, then let an agent group and describe it.
---

`miudb erd` introspects a live connection and writes a self-contained HTML
viewer plus `schema.json`, and optionally DBML. It supports SQLite, Postgres, and
MySQL; Snowflake and BigQuery return `erd.unsupported_db`. miudb never calls an
LLM here; an agent adds the polish through `meta.json`.

## Commands

```sh
miudb erd generate -c app                                   # index.html + schema.json
miudb erd generate -c app --format html,json,dbml --out-dir ./docs/erd
miudb erd generate -c app --meta ./docs/erd/meta.json       # with agent polish
miudb erd serve -c app                                      # render and open a browser
miudb erd serve --from ./docs/erd                           # an existing export, no database
miudb erd meta --stub -c app --out-dir ./docs/erd           # scaffold meta.json
```

| Flag | Commands | Notes |
|---|---|---|
| `-c`, `--connection` | all | Required, except `serve --from` |
| `-s`, `--schema` | all | Defaults to the connection's database. A connection without one errors with a "pass --schema" hint. |
| `--tables` | generate, serve | Comma-separated subset |
| `-m`, `--meta` | generate, serve | Path to `meta.json` |
| `-f`, `--format` | generate | `html`, `json`, `dbml`; default `html` (`schema.json` is always written) |
| `--out-dir` | generate, meta | See below |
| `--cdn` | generate, serve | Link renderer libraries from a CDN instead of inlining |
| `--title` | generate, serve | Used when `meta.title` is empty |
| `--from` | serve | Export directory or `schema.json` |
| `-p`, `--port` | serve | `0` picks a free port |
| `--no-open` | serve | Do not open the browser |
| `--stub`, `--force` | meta | `--stub` is required; `--force` overwrites |

Default `--out-dir`, first match wins: `$VD_VISUALS_PATH/<ref-slug>-erd`, then
`<git-root>/.work/visuals/<ref-slug>-erd` when `.work` exists, then
`.diagrams/<ref-slug>-erd`. The ref is flattened (`team/app` becomes
`team-app-erd`), so `team/app` and `team-app` share a folder; pass `--out-dir` to
keep them apart.

## The viewer

- Scroll to zoom, drag to pan, drag tables to move them (positions persist).
  Minimap, `Fit`, and `Re-layout` reframe.
- Click a table to spotlight its FK chain (depth selectable). Click a line to
  highlight both tables and the join columns. Double-click a table for the
  drawer: columns, types, PK/FK/audit, `ON DELETE`, incoming refs, indexes.
- Search tables and columns, filter by domain group, toggle framework tables,
  audit columns, and columns on nodes, hide single tables.
- **Find path**: shortest FK chain between two tables.
- **Insights**: missing PK, FK type mismatch, unindexed FK, orphans.
- **DBML**: the DBML source with a Copy button.
- **Share**: copies a link with filters and selection in the URL.
- The header counts visible and total tables, columns, and relationships. `?`
  lists shortcuts.

## Agent polish with meta.json

Without `meta.json` every table renders ungrouped and uncolored, and
`erd generate` warns. The recipe:

1. `miudb erd meta --stub -c app --out-dir <dir>` writes `meta.json` with
   detected `framework_tables` and `audit_columns`, an empty `groups`, and one
   empty `descriptions` entry per domain table.
2. Read `<dir>/schema.json`. FK topology, table-name prefixes, row counts, and
   migration file names are enough.
3. Put every non-framework table into 5-9 `groups`, one hex color each, and write
   one-line `descriptions` for the hub tables.
4. `miudb erd generate -c app --meta <dir>/meta.json`.

```json
{
  "title": "Shop",
  "database_type": "SQLite",
  "groups": {
    "Catalog": { "color": "#2563eb", "tables": ["products", "categories"] },
    "Sales": { "color": "#16a34a", "tables": ["orders", "order_items"] }
  },
  "framework_tables": ["migrations"],
  "audit_columns": ["created_at", "updated_at", "deleted_at"],
  "classifications": { "users": "PII" },
  "descriptions": { "orders": "Purchase orders placed by users" }
}
```

| Field | Purpose |
|---|---|
| `title` | Diagram heading |
| `database_type` | Label in the footer |
| `groups` | Colored domain swim-lanes |
| `framework_tables` | Hidden by default |
| `audit_columns` | Dimmed in column lists |
| `classifications` | Sensitivity tags such as `PII` |
| `descriptions` | One line per table in hover cards |

All fields are optional.
