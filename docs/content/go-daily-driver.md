---
title: "Daily driver"
---

miu-db uses a native Go store by default:

```text
~/.config/miu/db/connections.json
~/.config/miu/db/credentials.json
```

New connections should be created through `miudb connections add` so sensitive
fields are classified before write. The default secret store is the `miudb` OS
Keychain/keyring service. Migrated configs can keep `credentials-export.json`
in the same directory; `miudb` reads it when `credentials.json` is absent.

## Baseline

```bash
miudb connections list --output json
miudb connections smoke --timeout 20s --concurrency 4 --output json
```

:::note
Local-only connections are expected to fail when their databases are not
running. Remote and tunnel-backed failures should be compared against the same
network path from a known-good client before treating them as miu-db regressions.
:::

## Add Connections

SQLite:

```bash
miudb connections add \
  --name local-app \
  --db-type sqlite \
  --path /path/to/app.db \
  --output json
```

PostgreSQL/MySQL style TCP connection:

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

SSH tunnel-backed connection:

```bash
miudb connections add \
  --name app-prod \
  --db-type mysql \
  --host prod-rds.internal \
  --port 3306 \
  --database app \
  --username app \
  --password "$APP_DB_PASSWORD" \
  --tunnel \
  --ssh-config-alias bastion \
  --secret-store keyring \
  --output json
```

## Import / Share

Share a connections JSON file and import it on the target machine. Import merges
by name, overwrites matching entries, and backs up the existing
`connections.json` to `connections.json.bak-<timestamp>` before writing.

```bash
miudb connections import ./shared-connections.json --dry-run --output json
miudb connections import ./shared-connections.json --output json
```

:::caution
Secrets are imported verbatim. A file with inline passwords is self-contained
and needs no keyring setup on the target, but it carries plaintext credentials —
keep it mode `0600` and share over a secure channel only.
:::

## Query

```bash
miudb query run \
  --connection app-dev \
  --sql "select 1 as one" \
  --limit 100 \
  --output json
```
