<p align="center">
  <img src="assets/logo.png" alt="miu-db logo" width="180">
</p>

<h3 align="center">miu-db — query any database, from your terminal</h3>

<p align="center">
  <em>A fast, modal TUI for SQL databases. Vim keys, 25+ adapters, zero config.</em>
</p>

<p align="center">
  <a href="https://github.com/vanducng/miu-db/stargazers"><img src="https://img.shields.io/github/stars/vanducng/miu-db?style=flat&color=yellow" alt="GitHub Stars"></a>
  <img src="https://img.shields.io/badge/python-3.10+-blue.svg" alt="Python">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
</p>

<p align="center">
  <code>pipx install miu-db</code>
</p>

<p align="center">
  <a href="https://www.buymeacoffee.com/PeterAdams"><img src="https://img.shields.io/badge/Buy%20Me%20a%20Coffee-ffdd00?style=flat&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me a Coffee"></a>
</p>

---

### Connect
Supports all major databases: SQL Server, PostgreSQL, MySQL, SQLite, MariaDB, FirebirdSQL, Oracle, DuckDB, CockroachDB, ClickHouse, Snowflake, Supabase, CloudFlare D1, Turso, Athena, BigQuery, Spanner, RedShift, IBM Db2, SAP HANA, Teradata, Trino, Presto, Apache Flight SQL, Apache Impala, SurrealDB and osquery.

![Database Providers](docs/demos/demo-providers.gif)

### Query
Syntax highlighting. History. Vim-style keybindings.

![Query History](docs/demos/demo-history.gif)

### Results
Load millions of rows. Inspect data, filter by content, fuzzy search.

![Filter results](docs/demos/demo-filter/demo-filter.gif)

### Docker Discovery
Automatically finds running database containers. Press 'Enter' to connect, miu-db figures out the details for you.

![Docker Discovery](docs/demos/demo-docker-picker.gif)

---

## Features

**Connection manager:** Save and switch connections without CLI args

**Just run `miu-db`:** No CLI config needed, pick a connection and go

**Multi-database support:** PostgreSQL, MySQL, SQLite, SQL Server, and 10+ more

**Docker integration:** Auto-detect running database containers

**Cloud CLI integration:** Easily browse and connect to your external databases through Azure, AWS and GCP CLI's

**SSH tunnels:** Connect to remote databases securely with password or key auth

**Secure credentials:** Passwords stored in your OS keyring

**Vim-style editing:** Modal editing for terminal purists

**Query history:** Searchable, per-connection history

**Filter results:** Fuzzy search through millions of rows

**Context-aware help:** Keybindings shown on screen

**Browse databases:** Tables, views, procedures, indexes, triggers, sequences

**Autocomplete:** Sophisticated SQL completion engine for tables, columns, and procedures

**CLI mode:** Execute SQL from the command line

**Themes:** Rose Pine, Tokyo Night, Nord, Gruvbox

**Dependency wizard:** Auto-install missing drivers

---

## Motivation

Throughout my career, the undesputed truth was that heavy GUI's like SSMS was the only respectable way to access a database. It didn't matter that I wasn't a DBA, or that I didn't need complex performance graphs. I was expected to install a gigabyte-heavy behemoth that took ages to launch all for the mere purpose of running a few queries to update and view a couple of rows.

When I switched to Linux, I was suddenly unable to return to the devil I know, and I asked myself: _how do I access my data now?_

The popular answer was VS Code's SQL extension. But why should we developers launch a heavy Electron app designed for coding just to execute SQL?

I had recently grown fond of Terminal UI's for their speed and keybinding focus. I looked for SQL TUIs, but the options were sparse. The ones I found lacked the user-friendliness and immediate "pick-up-and-go" nature of tools I loved, like lazygit, and I shortly returning to vscode sql extension.

Something wasn't right. I asked myself, why is it that running SQL queries can't be enjoyable? So I created miu-db.

miu-db is for the developer who just wants to query their database with a user friendly UI without their RAM being eaten alive. It is a lightweight, beautiful, and keyboard-driven TUI designed to make accessing your data enjoyable, fast and easy like it should be-- all from inside your favorite terminal.

---

## Installation

> **Migrating from `sqlit-tui`?** Just `pip install miu-db` (or `pipx install miu-db`). First launch will copy your saved connections from `~/.config/sqlit/` and your keyring credentials to the new location automatically. Your old data stays in place as a safety backup. The `SQLIT_*` environment variables still work during the v1.x deprecation window.

```bash
# pipx (recommended)
pipx install miu-db

# uv
uv tool install miu-db

# pip
pip install miu-db

# Arch Linux (AUR)
yay -S miu-db

# Nix (flake)
nix run github:vanducng/miu-db
```

## Usage

```bash
miu-db
```

The keybindings are shown at the bottom of the screen.

### Try it without a database

Want to explore the UI without connecting to a real database? Run with mock data:

```bash
miu-db --mock=sqlite-demo
```

### CLI

```bash
miu-db -c "MyConnection"
miu-db --connection "MyConnection"

# Run a query
miu-db query -c "MyConnection" -q "SELECT * FROM Users"

# Output as CSV or JSON
miu-db query -c "MyConnection" -q "SELECT * FROM Users" --format csv
miu-db query -c "MyConnection" -f "script.sql" --format json

# Create connections for different databases
miu-db connections add mssql --name "MySqlServer" --server "localhost" --auth-type sql
miu-db connections add postgresql --name "MyPostgres" --server "localhost" --username "user" --password "pass"
miu-db connections add mysql --name "MyMySQL" --server "localhost" --username "user" --password "pass"
miu-db connections add cockroachdb --name "MyCockroach" --server "localhost" --port "26257" --database "defaultdb" --username "root"
miu-db connections add sqlite --name "MyLocalDB" --file-path "/path/to/database.db"
miu-db connections add turso --name "MyTurso" --server "libsql://your-db.turso.io" --password "your-auth-token"
miu-db connections add firebird --name "MyFirebird" --server "localhost" --username "user" --password "pass" --database "employee"
miu-db connections add athena --name "MyAthena" --athena-region-name "us-east-1" --athena-s3-staging-dir "s3://my-bucket/results/" --athena-auth-method "profile" --athena-profile-name "default"
miu-db connections add athena --name "MyAthenaKeys" --athena-region-name "us-east-1" --athena-s3-staging-dir "s3://my-bucket/results/" --athena-auth-method "keys" --username "ACCESS_KEY" --password "SECRET_KEY"

# Connect via SSH tunnel
miu-db connections add postgresql --name "RemoteDB" --server "db-host" --username "dbuser" --password "dbpass" \
  --ssh-enabled --ssh-host "ssh.example.com" --ssh-username "sshuser" --ssh-auth-type password --ssh-password "sshpass"

# Fetch password from a secrets manager (1Password, pass, Vault, etc.)
miu-db connections add postgresql --name "ProdDB" --server "prod.example.com" --username "dbuser" \
  --password-command "op read 'op://Work/prod-db/password'"

# Temporary (not saved) connection
miu-db connect sqlite --file-path "/path/to/database.db"

# Connect via URL - scheme determines database type (postgresql://, mysql://, sqlite://, etc.)
miu-db postgresql://user:pass@localhost:5432/mydb
miu-db mysql://root@localhost/testdb
miu-db sqlite:///path/to/database.db

# Save a connection via URL
miu-db connections add --url dbtype://user:pass@host/db --name "MyDB"

# Provider-specific CLI help
miu-db connect -h
miu-db connect supabase -h
miu-db connections add -h
miu-db connections add supabase -h

# Manage connections
miu-db connections list
miu-db connections delete "MyConnection"
```

## Keybindings

| Key | Action |
|-----|--------|
| `i` | Enter INSERT mode |
| `Esc` | Back to NORMAL mode |
| `e` / `q` / `r` | Focus Explorer / Query / Results |
| `Enter` (NORMAL) | Run the statement under the cursor (`Ctrl+Enter` in INSERT) |
| `s` | SELECT TOP 100 from table |
| `h` | Query history |
| `d` | Clear query |
| `n` | New query (clear all) |
| `y` | Copy query (when query editor is focused) |
| `v` / `y` / `Y` / `a` | View cell / Copy cell / Copy row / Copy all |
| `Ctrl+Q` | Quit |
| `?` | Help |

### Vim Motions (Query Editor, NORMAL mode)

Use with operators like `y`, `d`, `c` (e.g. `dw`, `y$`).

| Motion | Action |
|--------|--------|
| `h` / `j` / `k` / `l` | Move cursor left / down / up / right |
| `w` / `W` | Next word / WORD |
| `b` / `B` | Previous word / WORD |
| `0` / `$` | Line start / end |
| `gg` / `G` | File start / end |
| `f{c}` / `F{c}` | Find char forward / backward |
| `t{c}` / `T{c}` | Till char forward / backward |
| `%` | Matching bracket |

### Commands Menu (`<space>`)

| Key | Action |
|-----|--------|
| `<space>c` | Connect to database |
| `<space>x` | Disconnect |
| `<space>z` | Cancel running query |
| `<space>e` | Toggle Explorer |
| `<space>f` | Toggle Maximize |
| `<space>r` | Resize panes (then plain `←↑↓→`; any other key — incl. `Shift`/`Alt`+arrow — exits) |
| `<space>t` | Change theme |
| `<space>ga` / `<space>gr` | Run **all** statements in the buffer |
| `<space>gs` | Run statement at cursor (same as `Enter`) |
| `<space>h` | Help |
| `<space>q` | Quit |

Autocomplete triggers automatically in INSERT mode. Use `Tab` to accept.

---

## Configuration

Configuration lives in `$XDG_CONFIG_HOME/miu-db/` (default `~/.config/miu-db/`), overridable with `MIU_DB_CONFIG_DIR`. Legacy locations `~/.config/sqlit/` and `~/.sqlit/` are auto-migrated on first run (copy, not move — your old data is preserved). The `SQLIT_CONFIG_DIR` env var still works during the v1.x deprecation window.

### Customizing keybindings

miu-db reads optional overrides from `$XDG_CONFIG_HOME/miu-db/settings.json` (default `~/.config/miu-db/settings.json`). To swap the pane-focus keys (`e` / `q` / `r` → `1` / `2` / `3`), add:

```json
{
  "keymap": {
    "overrides": {
      "focus_explorer": "1",
      "focus_query": "2",
      "focus_results": "3"
    }
  }
}
```

Shortcut: `miu-db config edit` opens the file in `$EDITOR`. Inspect resolved bindings with `miu-db config show-keymap` — overridden rows are flagged with `*`.

Rebindable actions: `focus_explorer`, `focus_query`, `focus_results`, `resize_pane_{left,right,up,down}`. The resize actions ship without a default key (use `<space>r` mode by default) — bind `ctrl+arrow` if you'd rather skip the leader prefix:

```json
{
  "keymap": {
    "overrides": {
      "resize_pane_left":  "ctrl+left",
      "resize_pane_right": "ctrl+right",
      "resize_pane_up":    "ctrl+up",
      "resize_pane_down":  "ctrl+down"
    }
  }
}
```

Other keys (vim motions, chords, leader menus) stay fixed by design.

## FAQ

### How are sensitive credentials stored?

Connection details are stored in `connections.json` inside the config directory, but passwords are stored in your OS keyring when available (macOS Keychain, Windows Credential Locker, Linux Secret Service).

### How does miu-db compare to Harlequin, Lazysql, etc.?

miu-db is inspired by [lazygit](https://github.com/jesseduffield/lazygit) - you can just jump in and there's no need for external documentation. The keybindings are shown at the bottom of the screen and the UI is designed to be intuitive without memorizing shortcuts.

Key differences:
- **No need for external documentation** - miu-db embraces the "lazy" approach in that a user should be able to jump in and use it right away intuitively. There should be no setup instructions. If python packages are required for certain adapters, miu-db will help you install them as you need them.
- **No CLI config required** - Just run `miu-db` and pick a connection from the UI
- **Lightweight** - While Lazysql or Harlequin offer more features, I experienced that for the vast majority of cases, all I needed was a simple and fast way to connect and run queries. miu-db is focused on doing a limited amount of things really well.

---

## Inspiration

miu-db is built with [Textual](https://github.com/Textualize/textual) and inspired by:
- [lazygit](https://github.com/jesseduffield/lazygit) - Simple  TUI for git
- [lazysql](https://github.com/jorgerojas26/lazysql) - Terminal-based SQL client with connection manager

## Contributing

See `CONTRIBUTING.md` for development setup, testing, and CI steps.

### Driver Reference

Most of the time you can just run `miu-db` and connect. If a Python driver is missing, `miu-db` will show (and often run) the right install command for your environment.

| Database                            | Driver package               | `pipx`                                             | `pip` / venv                                       |
| :---------------------------------- | :--------------------------- | :------------------------------------------------- | :------------------------------------------------- |
| SQLite                              | *(built-in)*                 | *(built-in)*                                       | *(built-in)*                                       |
| PostgreSQL / CockroachDB / Supabase | `psycopg2-binary`            | `pipx inject miu-db psycopg2-binary`            | `python -m pip install psycopg2-binary`            |
| SQL Server                          | `mssql-python`               | `pipx inject miu-db mssql-python`               | `python -m pip install mssql-python`               |
| MySQL                               | `PyMySQL`                    | `pipx inject miu-db PyMySQL`                    | `python -m pip install PyMySQL`                    |
| MariaDB                             | `PyMySQL`                    | `pipx inject miu-db PyMySQL`                    | `python -m pip install PyMySQL`                    |
| Oracle                              | `oracledb`                   | `pipx inject miu-db oracledb`                   | `python -m pip install oracledb`                   |
| DuckDB                              | `duckdb`                     | `pipx inject miu-db duckdb`                     | `python -m pip install duckdb`                     |
| ClickHouse                          | `clickhouse-connect`         | `pipx inject miu-db clickhouse-connect`         | `python -m pip install clickhouse-connect`         |
| Turso                               | `libsql`                     | `pipx inject miu-db libsql`                     | `python -m pip install libsql`                     |
| Cloudflare D1                       | `requests`                   | `pipx inject miu-db requests`                   | `python -m pip install requests`                   |
| Snowflake                           | `snowflake-connector-python` | `pipx inject miu-db snowflake-connector-python` | `python -m pip install snowflake-connector-python` |
| Firebird                            | `firebirdsql`                | `pipx inject miu-db firebirdsql`                | `python -m pip install firebirdsql`                |
| Athena                              | `pyathena`                   | `pipx inject miu-db pyathena`                   | `python -m pip install pyathena`                   |
| BigQuery                            | `google-cloud-bigquery` + `google-cloud-bigquery-storage` + `pyarrow` | `pipx inject miu-db google-cloud-bigquery google-cloud-bigquery-storage pyarrow` | `python -m pip install google-cloud-bigquery google-cloud-bigquery-storage pyarrow` |
| Spanner                             | `google-cloud-spanner`       | `pipx inject miu-db google-cloud-spanner`       | `python -m pip install google-cloud-spanner`       |
| Apache Arrow Flight SQL             | `adbc-driver-flightsql`      | `pipx inject miu-db adbc-driver-flightsql`      | `python -m pip install adbc-driver-flightsql`      |
| Apache Impala                       | `impyla`                     | `pipx inject miu-db impyla`                     | `python -m pip install impyla`                     |
| SurrealDB                           | `surrealdb`                  | `pipx inject miu-db surrealdb`                  | `python -m pip install surrealdb`                  |
| osquery                             | `osquery`                    | `pipx inject miu-db osquery`                    | `python -m pip install osquery`                    |

### SSH Tunnel Support

SSH tunnel functionality requires additional dependencies. Install with the `ssh` extra:

```bash
# pipx
pipx install 'miu-db[ssh]'

# uv
uv tool install 'miu-db[ssh]'

# pip
pip install 'miu-db[ssh]'
```

If you try to create an SSH connection without these dependencies, miu-db will detect this and show you the exact command to install them for your environment.

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=vanducng/miu-db&type=Date)](https://star-history.com/#vanducng/miu-db&Date)

---

## License

MIT

---

## Acknowledgments

miu-db is a rebrand of [sqlit by Maxteabag](https://github.com/Maxteabag/sqlit).
Thanks to the upstream project for the foundation.
