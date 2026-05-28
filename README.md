<p align="center">
  <img src="assets/logo.png" alt="miudb logo" width="128">
</p>

<h1 align="center">miudb</h1>

<p align="center">Headless database CLI for agents, Neovim, and future terminal/web clients.</p>

`miudb` is the Go core for miu-db. It keeps the database layer focused on
connections, credentials, tunnels, adapters, query execution, schema inspection,
and machine-readable output. User interfaces live outside the core under
`ui/`.

## Install

```bash
brew install vanducng/tap/miudb
```

Or:

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.5
```

Verify:

```bash
miudb version --output json
miudb commands --output json
```

## Native Store

The default native config lives under:

```text
~/.config/miudb/connections.json
```

Sensitive values are classified before persistence. New connections store
database and SSH passwords outside `connections.json` by default using the
`miudb` OS Keychain/keyring service.

```bash
miudb connections add \
  --name local-app \
  --db-type sqlite \
  --path ./app.db \
  --secret-store keyring \
  --output json
```

```bash
miudb connections list --output json
miudb connections test local-app --output json
miudb query run --connection local-app --sql 'select 1 as one' --output json
```

Supported secret stores for new connections:

- `keyring`: OS Keychain/keyring service named `miudb` by default.
- `file`: `~/.config/miudb/credentials.json` with mode `0600`.
- `inline`: keep the value in the connection file.
- `none`: discard the supplied secret and require another resolver later.

Future secret providers can plug into the same `SecretRef` model, including
1Password and Bitwarden.

## Adapters

Daily-driver adapters in this preview:

- SQLite
- PostgreSQL
- MySQL
- Snowflake
- BigQuery

SSH tunnel-backed connections are supported for TCP adapters.

## UI

The Neovim scaffold lives in:

```text
ui/nvim/miu-db.nvim
```

Future clients should follow the same boundary:

```text
ui/nvim
ui/tui
ui/web
```

## Development

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands --output json
```

See [docs/golang-architecture.md](docs/golang-architecture.md) and
[docs/agent-cli.md](docs/agent-cli.md).
