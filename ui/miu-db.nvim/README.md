# miu-db.nvim

Minimal Neovim client for the Go `miudb` CLI.

This UI lives under `ui/` so future frontends can sit beside it:

```text
ui/miu-db.nvim
ui/tui
ui/web
```

```lua
vim.g.miu_db_connection = "agent-deck"
```

Open a normal `.sql` file and run:

```vim
:MiuDBSelectConnection
:MiuDBQuery
```

Or pass a connection:

```vim
:MiuDBQuery agent-deck
```

List saved connections:

```vim
:MiuDBConnections
```
