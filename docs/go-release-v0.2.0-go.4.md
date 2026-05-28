# miudb Go Preview v0.2.0-go.4

This release publishes the first Go preview of miu-db as `miudb`.

## Install

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.4
```

Or:

```bash
brew install vanducng/tap/miudb
```

## Highlights

- Adds the `miudb` Go CLI without replacing the Python `miu-db` TUI.
- Loads existing `/Users/vanducng/.config/miu/db` connections plus
  `credentials-export.json`.
- Supports SQLite, PostgreSQL, MySQL, Snowflake, and BigQuery daily-driver
  adapters.
- Supports SSH tunnel-backed connections.
- Provides JSON CLI envelopes for agent usage.
- Adds `connections smoke` for agent-readable saved-connection health checks.
- Adds JSON-RPC and NDJSON stdio protocol mode for future Neovim/TUI clients.
- Adds a minimal Neovim file-buffer client scaffold under `nvim/miu-db.nvim`.

## Verification

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
jq empty schemas/*.json testdata/contracts/*.json
```

Current local verification ignores unavailable local-only database services.
With those excluded, the saved-connection smoke matrix passes 21 of 23
connections from this environment. The two remaining non-local failures also
fail through installed Python `miu-db` here: `chat-prod` network timeout and
`wg` MySQL tunnel connection loss.
