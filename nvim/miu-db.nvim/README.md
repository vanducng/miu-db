# miu-db.nvim

Minimal Neovim client for the Go `miudb` CLI.

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
