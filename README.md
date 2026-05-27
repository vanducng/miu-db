<p align="center">
  <img src="https://raw.githubusercontent.com/vanducng/miu-db/main/assets/logo.png" alt="miu-db logo" width="128">
</p>

<h1 align="center">miu-db</h1>

<p align="center">
  <strong>Fast terminal SQL across your databases.</strong>
  <br>
  Pick a connection, write SQL, inspect results, stay in your shell.
</p>

<p align="center">
  <a href="https://github.com/vanducng/miu-db/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/vanducng/miu-db/ci.yml?branch=main" alt="CI"></a>
  <a href="https://pypi.org/project/miu-db/"><img src="https://img.shields.io/pypi/v/miu-db.svg" alt="PyPI"></a>
  <img src="https://img.shields.io/badge/python-3.10+-blue.svg" alt="Python">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
</p>

`miu-db` is part of the `miumono` umbrella: small, focused terminal-native
tools that are easy to install, easy to run locally, and predictable in CI.

## Quick Start

Install from PyPI:

```bash
uv tool install miu-db
miu-db
```

Try the TUI without a real database:

```bash
miu-db --mock=sqlite-demo
```

Run one query from the CLI:

```bash
miu-db query -c local -q "select 1"
```

## Source Install

For local development, install this checkout in editable mode. Code edits apply
the next time you run `miu-db`.

```bash
cd /Users/vanducng/git/personal/miu-db
uv tool install --force --editable ".[all]"
miu-db --mock=sqlite-demo
```

Re-run the install command after changing dependencies, entry points, or
`pyproject.toml`.

## Connections

Save local connections:

```bash
miu-db connections add sqlite --name local --file-path ./app.db
miu-db connections add postgresql --name pg --server localhost --database app --username app
miu-db connections add mysql --name mysql --server localhost --database app --username app
```

Connect without saving:

```bash
miu-db connect sqlite --file-path ./app.db
miu-db postgresql://user:pass@localhost:5432/app
miu-db mysql://root@localhost/app
```

Manage saved connections:

```bash
miu-db connections list
miu-db connections delete local
```

Passwords are stored in the OS keyring under the `miu-db` service when possible.

## Capabilities

- Terminal UI with explorer, SQL editor, and results grid.
- Vim-style normal/insert modes.
- Per-connection query history.
- Fuzzy filtering over result rows.
- Saved connections with keyring-backed passwords.
- Docker database discovery.
- SSH tunnels.
- Driver install hints when an adapter dependency is missing.

SQLite works out of the box. Optional adapters cover PostgreSQL, MySQL, SQL
Server, MariaDB, Oracle, DuckDB, ClickHouse, Snowflake, BigQuery, Athena,
Spanner, Turso, D1, Firebird, Redshift, Db2, HANA, Teradata, Trino, Presto,
Flight SQL, Impala, SurrealDB, osquery, and more.

## Keys

| Key | Action |
| --- | --- |
| `i` | Insert mode |
| `Esc` | Normal mode |
| `e` / `q` / `r` | Focus explorer / query / results |
| `Enter` | Run statement under cursor |
| `h` | Query history |
| `s` | Select top rows from table |
| `v` | View selected cell |
| `y` / `Y` | Copy cell / row |
| `<space>` | Command menu |
| `?` | Help |
| `Ctrl+Q` | Quit |

## Config

This is a brand-new `miu-db` app, not a legacy config alias.

- config: `~/.config/miu/db`
- override: `MIU_DB_CONFIG_DIR`
- env vars: `MIU_DB_*`
- keyring service: `miu-db`

```bash
miu-db config edit
miu-db config show-keymap
```

On first run, `miu-db` copies missing files from `~/.config/sqlit` into
`~/.config/miu/db` so existing saved connections keep working. Existing files
in the new location are never overwritten.

## Develop

```bash
uv sync --all-extras --dev
uv run miu-db --mock=sqlite-demo
uv run pytest tests/unit/test_config_dir_resolution.py -q
uv run pytest tests/test_sqlite.py -q --timeout=60
```

Build the package:

```bash
uv run python -m build
```

Release tags use miumono component SemVer:

```bash
miu-db-v0.1.5
```

## License

MIT
