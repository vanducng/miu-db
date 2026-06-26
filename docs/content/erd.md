---
title: "ERD diagrams"
---

`miudb erd` introspects a live database connection and produces an interactive
entity-relationship diagram as a self-contained HTML file, a `schema.json`
intermediate representation, and optionally DBML.

## Commands

### erd generate

Introspect a connection, write outputs to `--out-dir` (default resolved as:
`$VD_VISUALS_PATH/<conn-slug>-erd` → `<git-root>/.work/visuals/<conn-slug>-erd` when `.work` exists → `.diagrams/<conn-slug>-erd`),
and return an Envelope with `kind: erd.generate`. The connection ref is slugified into one
flat path segment (e.g. `prod/orders` → `prod-orders-erd`); an explicit `--out-dir` is used
verbatim and overrides this default. Slugification is lossy, so refs that differ only in
separator characters (`prod/orders` vs `prod-orders`) share one default folder — pass
`--out-dir` to disambiguate if you keep such connections side by side.

```
miudb erd generate --connection myconn --output json
miudb erd generate --connection myconn --meta .work/visuals/myconn-erd/meta.json
miudb erd generate --connection myconn --format html,json,dbml --out-dir ./docs/erd/
```

Flags: `--connection`, `--schema`, `--tables`, `--meta`, `--out-dir`,
`--format` (html,json,dbml), `--cdn`, `--title`. Short forms (all `erd`
commands): `-c`/`--connection`, `-s`/`--schema`, `-m`/`--meta`, `-f`/`--format`,
`-p`/`--port`. A connection with no default database errors with a clear
"pass --schema" hint.

### erd serve

Render the ERD and open it in a local browser. Accepts a live connection
(`--connection`) or an existing export directory (`--from`).

```
miudb erd serve --connection myconn
miudb erd serve --from .work/visuals/myconn-erd/
```

Flags: `--connection`, `--from`, `--schema`, `--tables`, `--meta`, `--port`,
`--no-open`, `--cdn`, `--title`.

### erd meta --stub

Scaffold a `meta.json` that an agent fills in. Introspects the schema
deterministically, detects framework tables, and writes a stub with every
domain table listed as an empty description.

```
miudb erd meta --stub --connection myconn --output json
miudb erd meta --stub --connection myconn --out-dir ./docs/erd/ --force
```

Flags: `--stub` (required), `--connection` (required), `--schema`, `--out-dir`,
`--force`.

Returns `kind: erd.meta` with `artifacts: [path/to/meta.json]`.

## Using the diagram

The generated `index.html` is a fully-offline interactive viewer:

- **Navigate** — scroll to zoom, drag the background to pan, drag a table to move
  it (positions persist); the minimap (top-right) and `Fit` / `Re-layout` reframe.
- **Explore relationships** — click a table to spotlight it and its FK chain
  (selectable depth); click a relationship line to highlight the two joined tables
  and their join columns; **double-click** a table for the details drawer (columns,
  types, PK/FK/audit, `ON DELETE`, incoming refs, indexes).
- **Filter** — live search (tables + columns); **Domain groups** checkboxes with
  **all** / **clear** to isolate one domain; toggle framework tables, audit columns,
  and `Columns on nodes`; hide individual tables (card `×`, restore via "show N hidden").
- **Find path** — shortest FK chain between two tables.
- **Insights** — lints the schema (missing PK, FK type mismatch, unindexed FK, orphans).
- **DBML** — opens the DBML source (dbdiagram.io / dbdocs format) with a Copy button.
- **Share** — copies a link with your filters and selection encoded in the URL.
- The header shows **visible/total** tables, columns, and relationships (live as you
  filter). Hover a truncated table/column name for the full value; `?` lists shortcuts.

## Two-layer model

```
Layer 1 — deterministic (miu-db)
  introspect schema → schema.json IR → HTML / DBML / JSON

Layer 2 — agentic (you / LLM)
  read schema.json → write meta.json → re-run erd generate --meta meta.json
```

miu-db never calls an LLM. An agent reads `schema.json`, authors `meta.json`,
then re-runs `erd generate` or `erd serve` with `--meta` to produce the polished
diagram.

## meta.json contract

```json
{
  "title": "My App",
  "database_type": "MySQL",
  "groups": {
    "Billing": { "color": "#f59e0b", "tables": ["invoices", "payments"] }
  },
  "framework_tables": ["migrations", "cache", "sessions"],
  "audit_columns": ["created_at", "updated_at", "deleted_at"],
  "classifications": { "users": "PII" },
  "descriptions": {
    "users": "Registered accounts",
    "orders": "Purchase orders placed by users"
  }
}
```

All fields are optional. The stub produced by `erd meta --stub` pre-populates
`database_type`, `audit_columns`, `framework_tables`, `groups` (empty map), and
`descriptions` (one key per domain table, value `""`).

| Field | Purpose |
|---|---|
| `title` | Diagram heading |
| `database_type` | Human label shown in the diagram footer |
| `groups` | Named clusters rendered as coloured swim-lanes |
| `framework_tables` | Hidden by default in the rendered view |
| `audit_columns` | Dimmed in column lists |
| `classifications` | Agent-applied sensitivity tags (PII, secret, …) |
| `descriptions` | Per-table prose shown in hover cards |

## Agent recipe

1. Run `miudb erd meta --stub --connection myconn` — creates
   `.diagrams/myconn-erd/meta.json`.
2. Read `.diagrams/myconn-erd/schema.json` (also written by `erd generate`).
3. Group tables by domain using FK topology, table name prefixes, and migration
   filenames. Assign a distinct `color` hex per group.
4. For each key in `descriptions`, write one sentence sourced from column names,
   FK targets, or migration filenames.
5. Remove any `framework_tables` entries that should remain visible (rare).
6. Re-run `miudb erd generate --connection myconn --meta
   .diagrams/myconn-erd/meta.json` to produce the polished HTML.
