---
title: "miudb Go Preview v0.2.0-go.6"
---

This release updates the Go preview of miu-db as `miudb`.

## Install

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.6
```

Or:

```bash
brew install vanducng/tap/miudb
```

## Highlights

- Keeps the branch clean Go-only for the `dev` target.
- Promotes the Go implementation as the active `main` branch.
- Uses `~/.config/miu/db` as the native miu product config store.
- Reads migrated `credentials-export.json` when `credentials.json` is absent.
- Classifies sensitive fields before persistence and stores new passwords in
  the `miudb` OS Keychain/keyring service by default.
- Supports SQLite, PostgreSQL, MySQL, Snowflake, and BigQuery daily-driver
  adapters.
- Supports SSH tunnel-backed connections.
- Provides JSON CLI envelopes for agent usage.
- Adds `connections smoke` for agent-readable saved-connection health checks.
- Adds JSON-RPC and NDJSON stdio protocol mode for future Neovim/TUI clients.
- Moves the minimal Neovim file-buffer client scaffold under
  `ui/miu-db.nvim`.
- Adds a Astro Starlight documentation site deployed to `miudb.vanducng.dev`.

## Verification

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb version --output json
./.miu-db/miudb connections add --secret-store file ...
./.miu-db/miudb query run --connection local --sql 'select 1 as one' --output json
```

The local CLI smoke uses a temporary native config directory and confirms file
secrets stay out of `connections.json`.
