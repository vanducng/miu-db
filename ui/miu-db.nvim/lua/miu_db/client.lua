local M = {}

local function decode(data)
  local ok, result = pcall(vim.json.decode, data)
  if ok then
    return result
  end
  return { ok = false, error = { message = result } }
end

function M.run(args, cb)
  local cmd = {
    "miudb",
    "--output",
    "json",
  }
  vim.list_extend(cmd, args)
  vim.system(cmd, { text = true }, function(obj)
    vim.schedule(function()
      cb(decode(obj.stdout or ""))
    end)
  end)
end

function M.connections(cb)
  M.run({ "connections", "list" }, cb)
end

function M.query(connection, sql, cb)
  M.run({
    "query",
    "run",
    "--connection",
    connection,
    "--sql",
    sql,
  }, cb)
end

return M
