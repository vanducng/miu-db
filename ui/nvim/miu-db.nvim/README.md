# miu-db.nvim

Minimal Neovim client for the Go `miudb` CLI.

This UI lives under `ui/nvim` so future frontends can sit beside it:

```text
ui/nvim
ui/tui
ui/web
```

```lua
vim.g.miu_db_connection = "agent-deck"
```

Open a normal `.sql` file and run:

```vim
:MiuDBQuery
```

Or pass a connection:

```vim
:MiuDBQuery agent-deck
```
