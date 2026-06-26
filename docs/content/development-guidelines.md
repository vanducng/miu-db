---
title: Development Guidelines
description: Local development and contribution rules for miu-db.
---

# Development Guidelines

## Local Checks

Run the Go test suite before pushing code changes:

```bash
go test ./...
```

Build a local binary without VCS stamping when working from a git worktree:

```bash
go build -buildvcs=false -o ./.miu-db/miudb ./cmd/miudb
./.miu-db/miudb version --output json
```

Validate the Neovim plugin Lua files after editing `ui/miu-db.nvim`:

```bash
luac -p ui/miu-db.nvim/lua/miu_db/*.lua ui/miu-db.nvim/plugin/miu_db.lua
```

## Code Boundaries

- `cmd/miudb` only wires the CLI entry point.
- `internal/cli` owns command definitions, output envelopes, and exit behavior.
- `internal/config` owns native config, secret refs, redaction, and migration
  compatibility.
- `internal/adapter` and `internal/adapters/*` own database-specific behavior.
- `internal/tunnel` owns SSH tunnel setup.
- `internal/query`, `internal/result`, and `internal/schema` own execution,
  pagination, and inspection.
- `ui/miu-db.nvim` is a client and should not import adapter internals.

## Secrets

:::danger
Never log or serialize raw credentials. Use the redaction helpers in
`internal/config` for CLI errors, connection output, and diagnostics. New
connections should store sensitive fields through `SecretRef` providers instead
of writing passwords inline.
:::

## Output Contract

Agent-facing commands should keep data on stdout and diagnostics on stderr.
JSON output should keep the envelope shape documented in
[CLI Contract](/cli-contract/), including stable `kind`, `command`,
`request_id`, `summary`, `data`, `warnings`, and structured errors.
