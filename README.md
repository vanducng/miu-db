<p align="center">
  <img src="assets/logo.png" alt="miudb logo" width="128">
</p>

<h1 align="center">miudb</h1>

<p align="center">A headless database CLI for humans and agents.</p>

`miudb` is the Go core for miu-db. It keeps the database layer focused on
connections, credentials, tunnels, adapters, query execution, schema inspection,
MCP, and machine-readable output.

## Install

```bash
brew install vanducng/tap/miudb
```

Or install with Go:

```bash
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0-go.9
```

Windows release archives are published on GitHub Releases as zip files. In
PowerShell:

```powershell
$version = "v0.2.0-go.9"
$asset = "miudb_windows_x86_64.zip"
Invoke-WebRequest "https://github.com/vanducng/miu-db/releases/download/$version/$asset" -OutFile $asset
Expand-Archive $asset -DestinationPath ".\miudb" -Force
.\miudb\miudb.exe version --output json
```

Use `miudb_windows_arm64.zip` on Windows ARM64.

Verify:

```bash
miudb version --output json
miudb commands --output json
```

## Native Store

The default native config lives under:

```text
~/.config/miu/db/connections.json
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
- `file`: `~/.config/miu/db/credentials.json` with mode `0600`.
- `inline`: keep the value in the connection file.
- `none`: discard the supplied secret and require another resolver later.

If `credentials.json` is absent, `miudb` also reads an existing
`credentials-export.json` in the same directory for migrated connections.

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

## MCP

Use miudb as a local stdio MCP server for coding-agent hosts:

```bash
miudb mcp serve --transport stdio
```

The MCP server exposes redacted connection inventory, schema inspection,
bounded read-only query execution, pagination, and `miudb://` resources. See
[docs/mcp.md](docs/mcp.md) for Codex, Claude Code, Cursor, and VS Code setup.

## Development

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands --output json
```

See [docs/golang-architecture.md](docs/golang-architecture.md) and
[docs/agent-cli.md](docs/agent-cli.md).
