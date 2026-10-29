---
title: Introduction
description: A headless database CLI for humans and agents.
---

<p><img src="/brand/banner.png" alt="miu-db" width="840" /></p>

[![Release](https://img.shields.io/github/v/release/vanducng/miu-db?label=release&color=7c3aed)](https://github.com/vanducng/miu-db/releases)

**miu-db** gives you and your coding agent one way into every database you use.
The `miudb` binary keeps saved connections, secrets, SSH tunnels, query paging,
schema inspection, and ERDs in one local Go core, and prints stable JSON on
stdout.

```sh
curl -fsSL https://db.miu.sh/install.sh | sh
miudb connections add --name app --db-type sqlite --path ./app.db
miudb query run --connection app --sql 'select 1 as one' -o pretty
```

![miudb demo: add a connection, smoke test, query, schema, ERD, and the activity log](/demo/miudb.gif)

New here? [Getting started](/onboarding/) takes five minutes.

## What it does

| Need | How | Page |
|---|---|---|
| Save and share connections | `connections add`, `import`; secrets in the keyring | [Connections and queries](/usage/) |
| Query safely | Bounded rows, cursor pages, per-call `--session` | [Connections and queries](/usage/#query) |
| Cloud warehouses | Snowflake OAuth or key pair, BigQuery ADC | [Authentication](/authentication/) |
| See a schema | `schema tree`, interactive offline ERD | [ERD diagrams](/erd/) |
| Give an agent tools | `miudb mcp serve`, read-only by default | [MCP server](/mcp/) |
| Audit what ran | JSONL activity log, no result rows | [Activity log](/activity/) |

Adapters: SQLite, PostgreSQL, MySQL, Snowflake, BigQuery. SSH tunnels for
Postgres and MySQL. A Neovim client lives in `ui/miu-db.nvim`.

## For agents

- Every command prints the stable `miudb.cli/v1` JSON envelope on stdout. See
  [Agent CLI contract](/cli-contract/).
- Every docs page is also raw Markdown: append `.md` to its URL, for example
  [`/onboarding.md`](/onboarding.md).
- Indexes: [`/llms.txt`](/llms.txt), [`/llms-full.txt`](/llms-full.txt).
- Skill: [`/skill.md`](/skill.md).

MIT. Source: [github.com/vanducng/miu-db](https://github.com/vanducng/miu-db).
