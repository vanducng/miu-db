local M = {}

local client = require("miu_db.client")
local buffers = require("miu_db.buffers")

function M.query_current_buffer(connection)
  connection = connection or vim.g.miu_db_connection
  if not connection or connection == "" then
    vim.notify("miu_db: set g:miu_db_connection or pass a connection name", vim.log.levels.ERROR)
    return
  end
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
  local sql = table.concat(lines, "\n")
  client.query(connection, sql, function(result)
    buffers.render_result(result)
  end)
end

return M
