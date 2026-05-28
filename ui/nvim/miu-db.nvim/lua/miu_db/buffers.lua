local M = {}

local function row_to_line(row)
  local values = {}
  for _, value in ipairs(row) do
    table.insert(values, tostring(value))
  end
  return table.concat(values, "\t")
end

function M.render_result(payload)
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_name(buf, "miudb://result")
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
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.cmd("botright split")
  vim.api.nvim_win_set_buf(0, buf)
end

return M
