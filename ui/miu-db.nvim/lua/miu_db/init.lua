local M = {}

local client = require("miu_db.client")
local buffers = require("miu_db.buffers")

function M.connections()
  client.connections(function(result)
    buffers.render_connections(result)
  end)
end

function M.select_connection()
  client.connections(function(result)
    if not result.ok then
      buffers.render_connections(result)
      return
    end
    local connections = (result.data and result.data.connections) or {}
    if #connections == 0 then
      vim.notify("miudb: no saved connections", vim.log.levels.WARN)
      return
    end
    vim.ui.select(connections, {
      prompt = "miudb connection",
      format_item = function(item)
        local endpoint = item.endpoint or {}
        local location = endpoint.host or endpoint.path or ""
        if endpoint.database and endpoint.database ~= "" then
          location = location .. "/" .. endpoint.database
        end
        return string.format("%s [%s] %s", item.name or "", item.db_type or "", location)
      end,
    }, function(choice)
      if not choice then
        return
      end
      vim.g.miu_db_connection = choice.name
      vim.notify("miudb: selected " .. choice.name, vim.log.levels.INFO)
    end)
  end)
end

function M.query_current_buffer(connection)
  connection = connection or vim.g.miu_db_connection
  if not connection or connection == "" then
    vim.notify("miudb: select a connection with :MiuDBSelectConnection or pass one to :MiuDBQuery", vim.log.levels.ERROR)
    return
  end
  local lines = vim.api.nvim_buf_get_lines(0, 0, -1, false)
  local sql = table.concat(lines, "\n")
  client.query(connection, sql, function(result)
    buffers.render_result(result)
  end)
end

return M
