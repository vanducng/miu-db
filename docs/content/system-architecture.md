---
title: System Architecture
description: Components and data flow in the Go miu-db core.
---

# System Architecture

## Component Flow

```text
Shell / agents        MCP hosts        Neovim SQL buffers        Future UI
      |                  |                    |                    |
      +---------- CLI / MCP / protocol boundary ------------------+
                                      |
                                  miu-db
                                      |
                config, secrets, tunnels, adapters, workers
                                      |
                                database drivers
```

## Core Responsibilities

The Go core owns saved connections, sensitive-field classification, secret
resolution, SSH tunnel setup, adapter selection, query execution, schema
inspection, result pagination, and machine-readable output.

Frontends are thin clients. The Neovim plugin shells out to `miudb`, renders
connection and result scratch buffers, and keeps SQL editing in normal files.
MCP hosts call the same core services through `miudb mcp serve --transport
stdio`; native `miudb serve --protocol jsonrpc|ndjson` remains available for
custom clients.

## Native Store

The default store is `~/.config/miu/db`:

```text
~/.config/miu/db/connections.json
~/.config/miu/db/credentials.json
```

For migrated configs, `credentials-export.json` is read from the same directory
when `credentials.json` is absent. New credentials default to the `miudb` OS
Keychain/keyring service.

## Runtime Paths

- `internal/config/store.go` resolves the default config directory and
  connection file.
- `internal/config/secrets.go` resolves keyring, file, gopass, command, and env
  secret providers.
- `internal/tunnel/tunnel.go` establishes SSH tunnel-backed TCP connections.
- `internal/cli/commands.go` exposes connection, query, schema, activity, and
  protocol commands.
- `internal/core/services.go` provides the shared service boundary used by CLI,
  native protocol, and MCP.
- `internal/mcpserver` exposes local stdio MCP tools and resources with
  allowlists, read-only defaults, output limits, and redacted errors.
- `internal/activity` captures per-operation JSONL events under
  `~/.config/miu/db/activity/{date}/{session_id}.jsonl` (SQL + metadata only,
  never result rows). Browseable via `miudb activity`; purgeable via
  `miudb activity prune`.
- `ui/miu-db.nvim` provides the current Neovim client.
