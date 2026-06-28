---
title: "Install"
---

The Go binary is named `miudb`.

## Release Install

One command, auto-detects OS and architecture (honors `MIUDB_VERSION` and
`MIUDB_INSTALL_DIR`):

```bash
curl -fsSL https://db.miu.sh/install.sh | sh
```

Alternatives:

```bash
brew install vanducng/tap/miudb
go install github.com/vanducng/miu-db/cmd/miudb@v0.2.0
```

Windows (PowerShell):

```powershell
irm https://db.miu.sh/install.ps1 | iex
```

Windows release archives are also published on GitHub Releases as zip files. In
PowerShell:

```powershell
$version = "v0.2.0"
$asset = "miudb_windows_x86_64.zip"
Invoke-WebRequest "https://github.com/vanducng/miu-db/releases/download/$version/$asset" -OutFile $asset
Expand-Archive $asset -DestinationPath ".\miudb" -Force
.\miudb\miudb.exe version --output json
```

Use `miudb_windows_arm64.zip` on Windows ARM64.

:::tip
Make sure your Go bin directory is on `PATH` when using `go install`:

```bash
export PATH="$(go env GOPATH)/bin:$PATH"
```
:::

Verify:

```bash
miudb version --output json
miudb commands --output json
```

## Native Config

By default `miudb` reads and writes native files under:

```text
~/.config/miu/db/connections.json
~/.config/miu/db/credentials.json
```

The connection file stores metadata. Sensitive fields are classified before
write and are stored through the selected secret backend.

Add a SQLite connection:

```bash
miudb connections add \
  --name local-app \
  --db-type sqlite \
  --path /path/to/app.db \
  --output json
```

Add a PostgreSQL connection with Keychain/keyring-backed password storage:

```bash
miudb connections add \
  --name app-dev \
  --db-type postgresql \
  --host localhost \
  --port 5432 \
  --database app \
  --username app \
  --password "$APP_DB_PASSWORD" \
  --secret-store keyring \
  --output json
```

List and test:

```bash
miudb connections list --output json
miudb connections test app-dev --output json
```

Run a bounded query:

```bash
miudb query run \
  --connection app-dev \
  --sql "select 1 as one" \
  --limit 100 \
  --output json
```

Run a saved-connection health matrix:

```bash
miudb connections smoke \
  --timeout 20s \
  --concurrency 4 \
  --output json
```

## Store Options

```bash
miudb --config-dir ~/.config/miu/db connections list --output json
miudb --connections-file ./connections.json connections list --output json
miudb --secret-source keyring,file,gopass connections test app-dev --output json
```

New connection secret stores:

- `keyring`: OS Keychain/keyring service named `miudb` by default.
- `file`: local credential file with mode `0600`.
- `inline`: leave the value in `connections.json`.
- `none`: discard the supplied value.

For migrated configs, `miudb` reads `credentials-export.json` from the same
directory when `credentials.json` is absent.

## Local Checkout

```bash
go test ./...
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb version --output json
```

`-buildvcs=false` avoids VCS stamping failures in git worktree layouts.
