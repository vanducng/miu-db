<p align="center">
  <img src="assets/logo.png" alt="miudb logo" width="128">
</p>

<h1 align="center">miudb</h1>

<p align="center">A headless database CLI for humans and agents.</p>

`miudb` is the Go core for miu-db. It keeps the database layer focused on
connections, credentials, tunnels, adapters, query execution, schema inspection,
MCP, and machine-readable output.

## Install

One command, auto-detects OS and architecture.

**macOS / Linux:**

```bash
curl -fsSL https://raw.githubusercontent.com/vanducng/miu-db/main/scripts/install.sh | sh
```

**Windows** (PowerShell):

```powershell
irm https://raw.githubusercontent.com/vanducng/miu-db/main/scripts/install.ps1 | iex
```

Both honor `MIUDB_VERSION` (pin a release) and `MIUDB_INSTALL_DIR`. Other options:

- **Homebrew** — `brew install vanducng/tap/miudb`
- **Go** — `go install github.com/vanducng/miu-db/cmd/miudb@latest`

Verify:

```bash
miudb version --output json
miudb commands --output json
```

Upgrade in place later (downloads the matching release, verifies its checksum,
and replaces the binary):

```bash
miudb upgrade            # to the latest release
miudb upgrade --check    # report whether a newer version exists
miudb upgrade --version v0.2.4
```

All commands take `-o`/`--output`: `json` (default, compact) or `pretty`.
Homebrew installs should upgrade with `brew upgrade miudb`.

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

## Share and Import

To onboard a new machine or teammate, share a connections JSON file and import
it. Import merges by name: matching connections are overwritten and the existing
`connections.json` is backed up first to `connections.json.bak-<timestamp>`.

```bash
miudb connections import ./shared-connections.json --output json
miudb connections import ./shared-connections.json --dry-run --output json
```

Secrets are imported as they appear in the file. A self-contained file with
inline passwords needs no extra setup on the target machine; treat it as a
secret (mode `0600`, secure channel only). Use `--dry-run` to preview the
added/overwritten connections before writing.

## Adapters

Daily-driver adapters:

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
bounded read-only query execution, pagination, and `miudb://` resources.

### Add to a host

**Claude Code** — one command:

```bash
claude mcp add --transport stdio miudb -- miudb mcp serve --transport stdio
```

**Cursor** — add to `~/.cursor/mcp.json` (global) or `.cursor/mcp.json` (project):

```json
{
  "mcpServers": {
    "miudb": { "command": "miudb", "args": ["mcp", "serve", "--transport", "stdio"] }
  }
}
```

**Codex** — add to `~/.codex/config.toml`:

```toml
[mcp_servers.miudb]
command = "miudb"
args = ["mcp", "serve", "--transport", "stdio"]
```

If the host can't find `miudb`, use the full path from `which miudb`. See
[the MCP docs](https://miudb.vanducng.dev/mcp/) for VS Code, per-connection
scoping, and tool reference.

## Development

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands --output json
```

See [the architecture docs](https://miudb.vanducng.dev/golang-architecture/) and
[the agent CLI docs](https://miudb.vanducng.dev/agent-cli/).
