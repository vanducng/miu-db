---
title: Tech Stack
description: Runtime, libraries, and release tooling used by miu-db.
---

# Tech Stack

## Runtime

- Go module: `github.com/vanducng/miu-db`
- Go toolchain: `1.25.8`
- CLI framework: Cobra
- JSON Schema validation in tests: `github.com/santhosh-tekuri/jsonschema/v6`

## Database Drivers

- SQLite: `modernc.org/sqlite`
- PostgreSQL: `github.com/jackc/pgx/v5/stdlib`
- MySQL: `github.com/go-sql-driver/mysql`
- Snowflake: `github.com/snowflakedb/gosnowflake`
- BigQuery: `cloud.google.com/go/bigquery`

## Secrets And Tunnels

- OS keychain/keyring: `github.com/99designs/keyring`
- SSH tunnels: `golang.org/x/crypto/ssh`
- Optional external secret lookup: `gopass` CLI

## Release And Docs

- Releases: GoReleaser
- Homebrew formula: `vanducng/homebrew-tap`
- Docs site: Astro Starlight
- Docs deploy: GitHub Pages through `.github/workflows/docs.yml`
