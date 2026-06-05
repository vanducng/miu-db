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

Use `--session key=value` (repeatable) to set temporary, per-call session context.
Keys are provider-specific and rejected when unsupported; they do not persist
across calls. For example, switch the Snowflake role for one query:

```bash
miudb query run --connection cnb-snowflake \
  --session role=DBT_ANALYTICS_ROLE \
  --sql 'select current_role()' --output json
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

## OAuth Authentication

Snowflake and BigQuery support token-based authentication with a one-time
interactive login. All subsequent queries refresh and use the stored token
silently.

**Snowflake** — add the connection once, then log in:

```bash
miudb connections add \
  --name sf-prod \
  --db-type snowflake \
  --host <account>.snowflakecomputing.com \
  --username <user> \
  --option authenticator=oauth \
  --option oauth_client_id=<client-id> \
  --option oauth_authorization_url=https://<account>.snowflakecomputing.com/oauth/authorize \
  --option oauth_token_request_url=https://<account>.snowflakecomputing.com/oauth/token-request \
  --option oauth_scope=session:role:ANALYST \
  --output json

miudb auth login sf-prod --output json
miudb auth status sf-prod --output json
miudb query run --connection sf-prod --sql 'SELECT CURRENT_ROLE()' --output json
```

No `--password` is required for OAuth connections.

**BigQuery** — authenticate once with gcloud, then add the connection:

```bash
gcloud auth application-default login

miudb connections add \
  --name bq-analytics \
  --db-type bigquery \
  --option project=my-gcp-project \
  --option dataset=analytics \
  --output json
```

See [the authentication docs](https://miudb.vanducng.dev/authentication/) for
the Snowflake `CREATE SECURITY INTEGRATION` snippet, all option keys, and
secret-handling guarantees.

## Adapters

Daily-driver adapters:

- SQLite
- PostgreSQL
- MySQL
- Snowflake (password, JWT, OAuth)
- BigQuery (ADC, service account)

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

## Activity Log

Every query, exec, and schema operation is recorded as a JSONL event under:

```text
~/.config/miu/db/activity/{date}/{session_id}.jsonl
```

Each line captures SQL text, `sql_shape` (literals replaced with `?`),
connection name, group, db\_type, latency, rows\_returned count, and error
info. **Result rows are never stored** — only counts and metadata.

### Browse logs

```bash
# all events from the last day
miudb activity --since 1d --output json

# all events for a specific connection
miudb activity --connection local-app --since 7d --output json

# only failures
miudb activity --failed --since 24h --output json

# reconstruct a full session trace (spans multiple date dirs if needed)
miudb activity --session <session_id> --output json
```

### Prune old logs

```bash
miudb activity prune --older-than 30d          # delete dirs older than 30 days
miudb activity prune --older-than 30d --dry-run # preview without deleting
```

### Opt out

Disable for a single invocation:

```bash
miudb query run --connection local-app --sql 'select 1' --no-activity-log
```

Disable globally via environment variable:

```bash
MIUDB_ACTIVITY_LOG=off miudb query run --connection local-app --sql 'select 1'
```

Disable per-connection by setting `"log_sql": false` in `connections.json`:

```json
{ "name": "local-app", "db_type": "sqlite", "log_sql": false, ... }
```

With `log_sql: false`, `sql_shape` (normalized, no literals) is still
recorded; only the raw SQL text is omitted.

### `--session` flag distinction

`query run --session key=value` sets a temporary, per-call connection context
(e.g. Snowflake role). This is unrelated to the activity log session\_id, which
is a machine-generated identifier minted once per CLI invocation or MCP server
start. The flags share a name for historical reasons; a rename is a separate
decision tracked in `plans/260605-1736-session-activity-store/decisions.md`.

### Power-user SQL path (DuckDB)

For ad-hoc analytics across all sessions, load the raw JSONL directly in DuckDB:

```sql
select session_id, connection, op, sql_shape, latency_ms, rows_returned, ts
from read_json_auto('~/.config/miu/db/activity/**/*.jsonl')
order by ts desc
limit 100;
```

## Development

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands --output json
```

See [the architecture docs](https://miudb.vanducng.dev/golang-architecture/) and
[the agent CLI docs](https://miudb.vanducng.dev/agent-cli/).
