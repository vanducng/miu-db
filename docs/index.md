---
title: Home
description: Headless database CLI for humans and agents.
---

# miu-db

`miudb` is a headless database CLI for humans and agents. It keeps database
connections, secrets, tunnels, adapters, query execution, schema inspection,
and machine-readable output in a Go core that can be driven from the shell,
Neovim, or future UIs.

## Start Here

- [Install miudb](go-install.md)
- [Daily driver notes](go-daily-driver.md)
- [Agent CLI usage](agent-cli.md)
- [CLI contract](cli-contract.md)

## Core Model

- The Go CLI is named `miudb`.
- Native config lives under `~/.config/miu/db`.
- Sensitive values are classified before persistence.
- New credentials use the `miudb` OS Keychain/keyring service by default.
- Migrated file credentials can stay in `credentials-export.json`.
- SSH tunnel-backed connections are first-class for TCP adapters.

## Interfaces

- CLI and agents use JSON envelopes on stdout.
- Neovim uses normal `.sql` buffers through `ui/miu-db.nvim`.
- Future UIs should sit under `ui/` and call the core through the same
  command/protocol boundary.

## Supported Daily Drivers

- SQLite
- PostgreSQL
- MySQL
- Snowflake
- BigQuery
