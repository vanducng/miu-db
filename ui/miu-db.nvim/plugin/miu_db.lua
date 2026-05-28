if vim.g.loaded_miu_db == 1 then
  return
end
vim.g.loaded_miu_db = 1

vim.api.nvim_create_user_command("MiuDBQuery", function(opts)
  require("miu_db").query_current_buffer(opts.args)
end, { nargs = "?" })

vim.api.nvim_create_user_command("MiuDBConnections", function()
  require("miu_db").connections()
end, {})

vim.api.nvim_create_user_command("MiuDBSelectConnection", function()
  require("miu_db").select_connection()
end, {})
