# miudb Daily Driver Notes

This preview uses a native Go store by default:

```text
~/.config/miudb/connections.json
~/.config/miudb/credentials.json
```

New connections should be created through `miudb connections add` so sensitive
fields are classified before write. The default secret store is the `miudb` OS
Keychain/keyring service.

## Baseline

```bash
miudb connections list --output json
miudb connections smoke --timeout 20s --concurrency 4 --output json
```

Local-only connections are expected to fail when their databases are not
running. Remote and tunnel-backed failures should be compared against the same
network path from a known-good client before treating them as miudb regressions.

## Add Connections

SQLite:

```bash
miudb connections add \
  --name agent-deck \
  --db-type sqlite \
  --path /Users/vanducng/.agent-deck/profiles/default/state.db \
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

## Query

```bash
miudb query run \
  --connection app-dev \
  --sql "select 1 as one" \
  --limit 100 \
  --output json
```
