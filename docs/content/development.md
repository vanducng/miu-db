---
title: Development
description: Build, test, and release miu-db, and deploy the docs site.
---

## Local checks

```sh
go test ./...
go test ./tests/go -run TestMCPCommandTransportSQLiteFlow -count=1
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb commands
```

CI (`go-ci.yml`) runs the same tests with `GOFLAGS=-buildvcs=false`. Live-DB
tests read a DSN from `MIUDB_TEST_MYSQL_DSN` or `MIUDB_TEST_PG_DSN` and skip
when it is unset.

After editing the Neovim plugin:

```sh
luac -p ui/miu-db.nvim/lua/miu_db/*.lua ui/miu-db.nvim/plugin/miu_db.lua
```

Before a release, confirm stdout stays clean when MCP fails to start:

```sh
./.miu-db/miudb mcp serve --transport bad 1>/tmp/miudb-mcp.out 2>/tmp/miudb-mcp.err || test $? -eq 2
test ! -s /tmp/miudb-mcp.out
```

## Contract rules

- Data on stdout, diagnostics on stderr. Keep the envelope fields in
  [Agent CLI contract](/cli-contract/) stable: `kind`, `command`, `request_id`,
  `summary`, `data`, `warnings`, and typed `error`.
- Never log or serialize raw credentials. Use the redaction helpers in
  `internal/config`.
- Tests and docs use synthetic `users` / `products` / `orders` data only.
- A change to commands, flags, or output updates the skill at
  `.agents/skills/miudb/SKILL.md` in the same change.

## Release

release-please opens a release PR from conventional commits on `main` and merges
it; the tag then runs GoReleaser in the same workflow. GoReleaser builds static
Darwin, Linux, and Windows archives (`.tar.gz`, Windows `.zip`) with
`checksums.txt`, publishes a GitHub release, and updates the Homebrew tap
`vanducng/tap/miudb` when `HOMEBREW_TAP_GITHUB_TOKEN` is set.

The `main` branch is the Go implementation; `python-version` keeps the old
Python one.

## Docs

```sh
cd docs && npm ci && npm run build   # astro build -> docs/dist/
```

`docs.yml` copies `scripts/install.sh`, `scripts/install.ps1`, and the skill to
the site root, builds `docs/` with `withastro/action`, and deploys to GitHub
Pages (custom domain from `docs/public/CNAME`). Every page is also served as
Markdown at `/<page>.md`.
