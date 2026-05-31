---
title: Home
description: Headless database CLI for humans and agents.
---

<section class="miu-hero">
  <div class="miu-kicker">miu-db</div>
  <h1>Headless database work for humans and agents.</h1>
  <p>
    <code>miudb</code> keeps connections, secrets, tunnels, adapters, query
    execution, schema inspection, and machine-readable output in a Go core
    that can be driven from the shell, Neovim, or future UIs.
  </p>
  <div class="miu-actions">
    <a class="miu-button miu-button-primary" href="go-install/">Install miudb</a>
    <a class="miu-button miu-button-secondary" href="agent-cli/">Agent CLI</a>
  </div>
</section>

<section class="miu-card-grid">
  <a class="miu-card" href="go-daily-driver/">
    <span>01</span>
    <h2>Daily Driver</h2>
    <p>SQLite, PostgreSQL, MySQL, Snowflake, BigQuery, and SSH tunnels.</p>
  </a>
  <a class="miu-card" href="system-architecture/">
    <span>02</span>
    <h2>Headless Core</h2>
    <p>Config, secrets, adapters, tunnels, workers, and result paging.</p>
  </a>
  <a class="miu-card" href="cli-contract/">
    <span>03</span>
    <h2>Agent Contract</h2>
    <p>Stable JSON envelopes, bounded query output, and redacted errors.</p>
  </a>
  <a class="miu-card" href="mcp/">
    <span>04</span>
    <h2>MCP Server</h2>
    <p>Local stdio tools and resources for Codex, Claude Code, Cursor, and VS Code.</p>
  </a>
</section>

## Core Model

- The Go CLI is named `miudb`.
- Native config lives under `~/.config/miu/db`.
- Sensitive values are classified before persistence.
- New credentials use the `miudb` OS Keychain/keyring service by default.
- Migrated file credentials can stay in `credentials-export.json`.
- SSH tunnel-backed connections are first-class for TCP adapters.

## Interfaces

- CLI and agents use JSON envelopes on stdout.
- MCP hosts use `miudb mcp serve --transport stdio`.
- Neovim uses normal `.sql` buffers through `ui/miu-db.nvim`.
- Future UIs should sit under `ui/` and call the core through the same
  command/protocol boundary.
