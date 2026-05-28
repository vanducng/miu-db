---
title: System Architecture
description: Components and data flow in the Go miudb core.
---

# System Architecture

## Component Flow

```text
Shell / agents        Neovim SQL buffers        Future UI
      |                      |                     |
      +---------- CLI / protocol boundary ---------+
                             |
                          miudb
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
- `internal/cli/commands.go` exposes connection, query, schema, and protocol
  commands.
- `ui/miu-db.nvim` provides the current Neovim client.
