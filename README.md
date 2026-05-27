<p align="center">
  <img src="assets/hero.png" alt="miu-db hero" width="860">
</p>

<h1 align="center">miu-db</h1>

<p align="center">
  Database TUI for the <strong>miumono</strong> tool family. Open a terminal, pick a connection, run SQL.
</p>

<p align="center">
  <a href="https://github.com/vanducng/miu-db/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/vanducng/miu-db/ci.yml?branch=main" alt="CI"></a>
  <img src="https://img.shields.io/badge/python-3.10+-blue.svg" alt="Python">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
</p>

## Install

Install the latest package:

```bash
uv tool install miu-db
```

Install from this checkout in editable mode, so source edits apply on the next run:

```bash
cd /Users/vanducng/git/personal/miu-db
uv tool install --force --editable ".[all]"
```

Other install options:

```bash
pipx install miu-db
pip install miu-db
nix run github:vanducng/miu-db
```

## Run

```bash
miu-db
```

Try it without a database:

```bash
miu-db --mock=sqlite-demo
```

Run SQL from the command line:

```bash
miu-db query -c "local" -q "select 1"
miu-db query -c "local" -f ./query.sql --format csv
```

## Connections

Save a connection:

```bash
miu-db connections add sqlite --name local --file-path ./app.db
miu-db connections add postgresql --name pg --server localhost --database app --username app
miu-db connections add mysql --name mysql --server localhost --database app --username app
```

Connect once without saving:

```bash
miu-db connect sqlite --file-path ./app.db
miu-db postgresql://user:pass@localhost:5432/app
miu-db mysql://root@localhost/app
```

List or remove saved connections:

```bash
miu-db connections list
miu-db connections delete local
```

Passwords are stored in the OS keyring under the `miu-db` service when possible.

## What It Does

- Explorer, query editor, and results grid in one terminal screen.
- Vim-style normal/insert modes.
- Per-connection query history.
- Fuzzy filtering over result rows.
- Secure saved connections with keyring-backed passwords.
- Docker database discovery.
- SSH tunnels.
- Driver install hints when an adapter dependency is missing.

Built-in SQLite works immediately. Optional adapters cover PostgreSQL, MySQL, SQL Server, MariaDB, Oracle, DuckDB, ClickHouse, Snowflake, BigQuery, Athena, Spanner, Turso, D1, Firebird, Redshift, Db2, HANA, Teradata, Trino, Presto, Flight SQL, Impala, SurrealDB, osquery, and more.

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

miu-db is a brand-new miumono component. It only reads miu-db names:

- config: `~/.config/miu-db`
- override: `MIU_DB_CONFIG_DIR`
- env vars: `MIU_DB_*`
- keyring service: `miu-db`

Edit settings:

```bash
miu-db config edit
miu-db config show-keymap
```

## Develop

```bash
uv sync --all-extras --dev
uv run miu-db --mock=sqlite-demo
uv run pytest tests/unit/test_config_dir_resolution.py -q
uv run pytest tests/test_sqlite.py -q --timeout=60
```

Build:

```bash
uv run python -m build
```

Release tags use the miumono component format:

```bash
miu-db-v0.1.0
```

## License

MIT
