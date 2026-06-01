---
title: Deployment
description: Release, Homebrew, and documentation deployment process.
---

# Deployment

## Branches

- `main` is the Go implementation and the active development branch.
- `python-version` preserves the previous Python implementation.

## CLI Release

Releases are tag-driven. Push a version tag to run GoReleaser:

```bash
git tag -a v0.2.0-go.9 -m "miudb v0.2.0-go.9"
git push origin v0.2.0-go.9
```

The release workflow builds Darwin, Linux, and Windows archives, publishes a
GitHub release, and updates the Homebrew tap when the
`HOMEBREW_TAP_GITHUB_TOKEN` repository secret is configured. Windows artifacts
are published as zip archives.

## Homebrew Install

```bash
brew install vanducng/tap/miudb
brew upgrade vanducng/tap/miudb
miudb version --output json
```

## Documentation Deploy

Documentation deploys from `main` through GitHub Pages:

```bash
zensical build --clean
```

The docs workflow writes `miu-db.vanducng.dev` into `site/CNAME`, uploads the
generated `site/` directory, and deploys it through `actions/deploy-pages`.

## MCP Release Check

The Homebrew and `go install` artifacts include the local stdio MCP server; no
extra runtime dependencies are required after `miudb` is on `PATH`.

Before release, verify:

```bash
go test ./...
go test ./tests/go -run TestMCPCommandTransportSQLiteFlow -count=1
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb mcp serve --transport bad 1>/tmp/miudb-mcp.out 2>/tmp/miudb-mcp.err || test $? -eq 2
test ! -s /tmp/miudb-mcp.out
```

`miudb mcp serve --transport stdio` is the supported MCP entry point. Native
`miudb serve --protocol jsonrpc|ndjson` remains for Neovim and custom clients.
