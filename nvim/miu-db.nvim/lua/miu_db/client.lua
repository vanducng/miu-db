local M = {}

local function decode(data)
  local ok, result = pcall(vim.json.decode, data)
  if ok then
    return result
  end
  return { ok = false, error = { message = result } }
end

function M.query(connection, sql, cb)
  local cmd = {
    "miudb",
    "query",
    "run",
    "--connection",
    connection,
    "--sql",
    sql,
    "--output",
    "json",
  }
  vim.system(cmd, { text = true }, function(obj)
    vim.schedule(function()
      cb(decode(obj.stdout or ""))
    end)
  end)
end

return M
