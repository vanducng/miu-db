local M = {}

local function row_to_line(row)
  local values = {}
  for _, value in ipairs(row) do
    if value == vim.NIL or value == nil then
      table.insert(values, "NULL")
    else
      table.insert(values, tostring(value))
    end
  end
  return table.concat(values, "\t")
end

local function open_scratch(name, lines)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_name(buf, name)
  vim.bo[buf].buftype = "nofile"
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].filetype = "miudb"
  vim.bo[buf].swapfile = false
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.cmd("botright split")
  vim.api.nvim_win_set_buf(0, buf)
end

function M.render_result(payload)
  local lines = {}
  if not payload.ok then
    table.insert(lines, "ERROR")
    table.insert(lines, payload.error and payload.error.message or "unknown error")
  else
    local result = payload.data and payload.data.result
    if result and result.columns then
      local headers = {}
      for _, col in ipairs(result.columns) do
        table.insert(headers, col.name)
      end
      table.insert(lines, table.concat(headers, "\t"))
      for _, row in ipairs(result.rows or {}) do
        table.insert(lines, row_to_line(row))
      end
    else
      table.insert(lines, vim.inspect(payload.data))
    end
  end
  open_scratch("miudb://result", lines)
end

function M.render_connections(payload)
  local lines = {}
  if not payload.ok then
    table.insert(lines, "ERROR")
    table.insert(lines, payload.error and payload.error.message or "unknown error")
  else
    table.insert(lines, "name\tdb_type\tfolder\thost\tdatabase")
    for _, conn in ipairs((payload.data and payload.data.connections) or {}) do
      local endpoint = conn.endpoint or {}
      table.insert(lines, table.concat({
        conn.name or "",
        conn.db_type or "",
        conn.folder_path or "",
        endpoint.host or endpoint.path or "",
        endpoint.database or "",
      }, "\t"))
    end
  end
  open_scratch("miudb://connections", lines)
end

return M
