if vim.g.loaded_miu_db == 1 then
  return
end
vim.g.loaded_miu_db = 1

vim.api.nvim_create_user_command("MiuDBQuery", function(opts)
  require("miu_db").query_current_buffer(opts.args)
end, { nargs = "?" })
