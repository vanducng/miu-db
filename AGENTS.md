# AGENTS.md

Operational rules for AI agents working in this repository.

## Privacy — never commit personal or work data

This is a public, general-purpose database CLI. **Do not leak the maintainer's
real database connections, schemas, or infrastructure into the repository.**

- **No real connection identifiers.** Saved connections (`miudb connections list`)
  are private. Never copy their names, groups, `group/name` refs, hostnames,
  endpoints, usernames, ports, or tunnel hosts into code, tests, fixtures, docs,
  comments, commit messages, or PR descriptions.
- **No real schemas.** Test fixtures and examples use **synthetic** data only —
  generic `users` / `products` / `orders` / `categories` shop or blog schemas.
  Never commit a real introspected schema, table list, column names, row counts,
  or descriptions taken from a live or work database.
- **Integration tests stay env-gated and DB-agnostic.** Live-DB tests read a DSN
  from an env var (`MIUDB_TEST_MYSQL_DSN`, `MIUDB_TEST_PG_DSN`) and `t.Skip` when
  unset. They assert structural sanity (table/column/FK shape) — never compare
  against a specific named or personal database.
- **Invent generic names for examples.** When a PR, doc, or test needs sample
  output, make up generic names; do not paste real ones from the environment.

Before committing, scan the diff for connection/host/schema identifiers that look
real and replace them with synthetic equivalents.

## Keep the downstream skill in sync

This CLI has a downstream consumer: the **`miudb` skill** at
`~/skills/skills/miudb/SKILL.md` (Claude Code skill). After landing a change here,
check whether the skill needs updating and ship it in the `vanducng/skills` repo:

- **New/renamed commands, flags, or output shape** → update the skill's command
  examples and the "Output contract" / discovery sections.
- **ERD renderer template** (`internal/erd/assets/erd-template.html`) is mirrored
  **byte-for-byte** into the `diagram` skill's `er_html.py` `_TEMPLATE`. Any edit
  here must be re-synced there (regex-replace the `_TEMPLATE = r"""…"""` block,
  then `py_compile` + run the skill's tests). The two must stay identical.
- **New ERD viewer features** → document them in the skill's ERD "Viewer" section.

Rule of thumb: if a change alters what a user types or sees, the skill is likely
stale — update it in the same work session, don't defer.
