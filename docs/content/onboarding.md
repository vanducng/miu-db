---
title: Getting started
description: Install miudb and its agent skill, then let your agent add a connection and run the first query.
---

## 1. Install

```sh
curl -fsSL https://db.miu.sh/install.sh | sh
curl -fsSL https://db.miu.sh/skill.md --create-dirs \
  -o ~/.claude/skills/miudb/SKILL.md
```

The first line installs the `miudb` binary (checksum verified, honors
`MIUDB_VERSION` and `MIUDB_INSTALL_DIR`). The second adds the agent skill. For
Codex, Cursor, and other agents, save it to `~/.agents/skills/miudb/SKILL.md`
instead. Homebrew, Windows, and `go install` are on [Install](/install/).

## 2. Ask your agent

Paste this into Claude Code, Codex, or any coding agent:

```text
/miudb set up miudb: list my saved connections, add a SQLite connection for
./app.db if none exists, smoke-test everything, then show me the schema.
```

No skill installed? Any agent that can fetch URLs works too:

```text
Read https://db.miu.sh/skill.md and follow it to set up miudb, add a SQLite
connection for ./app.db, and show me its schema.
```

## 3. Or do it by hand

![miudb demo: add a connection, smoke test, query, schema, ERD, and the activity log](/demo/miudb.gif)

```sh
miudb connections add --name app --db-type sqlite --path ./app.db
miudb query run --connection app --sql 'select 1 as one' -o pretty
```

Postgres, MySQL, SSH tunnels, Snowflake, and BigQuery are on
[Connections and queries](/usage/) and [Authentication](/authentication/).

## Everyday commands

| Goal | Command |
|---|---|
| Add a SQLite file | `miudb connections add --name app --db-type sqlite --path ./app.db` |
| Add Postgres (password to keyring) | `miudb connections add --name app --db-type postgresql --host localhost --port 5432 --database app --username app --password "$APP_DB_PASSWORD"` |
| Run one statement | `miudb query run --connection app --sql 'select * from users' --limit 50` |
| Next page of a result | `miudb query fetch-page --cursor <page.next_cursor>` |
| Tables and columns | `miudb schema tree --connection app` |
| Interactive ERD | `miudb erd serve -c app` |
| Import a teammate's connections | `miudb connections import ./shared.json --dry-run` |

## Probe setup and usage

| Question | Command |
|---|---|
| Installed version, any update? | `miudb version` / `miudb upgrade --check` |
| Which connections exist? | `miudb connections list --basic -o pretty` |
| Is this one reachable? | `miudb connections test app` |
| Which ones are broken? | `miudb connections smoke --timeout 20s` |
| What did I or my agent run? | `miudb activity --since 24h -o pretty` |
| Only the failures | `miudb activity --failed --since 7d` |
| Which commands exist, machine-readable? | `miudb commands` / `miudb describe query run` |
| Any flag | `miudb <command> --help` |

Every command prints the `miudb.cli/v1` JSON envelope by default. Use
`-o pretty` to read it yourself; agents should parse the JSON.

## Next

- [Connections and queries](/usage/): secret stores, tunnels, import, sessions, scripts, paging.
- [MCP server](/mcp/): give Claude Code, Codex, Cursor, or VS Code read-only database tools.
- [ERD diagrams](/erd/): offline interactive diagrams your agent can polish.
- [Agent CLI contract](/cli-contract/): envelope, exit codes, redaction.
- Agents: append `.md` to any page URL for raw Markdown, for example
  [`/usage.md`](/usage.md), or start from [`/llms.txt`](/llms.txt).
