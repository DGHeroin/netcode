package netcode

var _context_init_script = `
netcode = {}
-- 各种接口, 使用local获得接口后, 覆盖原全局变量
local nc_start = netcode_start
netcode_start = nil
local nc_call = netcode_call
netcode_call = nil
local nc_exit = netcode_exit
netcode_call = nil
local nc_rpc_serve = netcode_rpc_serve
netcode_rpc_serve = nil
local nc_rpc_client = netcode_rpc_client
netcode_rpc_client = nil
local nc_tick = netcode_tick
netcode_tick = nil
local nc_reply = netcode_reply
netcode_reply = nil


netcode.time = 0
netcode.deltaTime = 0
netcode.frameCount=0
local tick_list = {}
nc_tick(function(t,dt)
    netcode.frameCount = netcode.frameCount + 1
    netcode.time = t
    netcode.deltaTime = dt
    for i, cb in pairs(tick_list) do
        cb()
    end
end)
--
-- 一些初始化

function netcode.start(name, cb)
    if type(name) == 'function' then cb, name = name, '' end
    nc_start(name, cb)
end
local log_index=0
local o_print = print
function netcode.log(...)
    log_index = log_index+1
    o_print(string.format('[%08x]', log_index), ...)
end
print = netcode.log
function netcode.on_tick(cb)
    table.insert(tick_list, cb)
end

function netcode.add_updater(period)
    period = period or 0
    local last_time = netcode.time
    local update_list = {}
    local fn = function()
        if netcode.time - last_time <= period then return end
        last_time = netcode.time
        for i, cb in pairs(update_list) do
            cb()
        end
    end
    netcode.on_tick(fn)
    return {
        add =  function(cb)
            table.insert(update_list, cb)
        end
    }
end
local mailList = {}
local mailId = 0
local function getMailId()
    while true do
        mailId = mailId + 1
        if not mailList[mailId] then return mailId end
    end
end
-- 注册 消息回复
nc_reply(function(mailId, err, ...)
    mailId = math.ceil(mailId)
    local fn = mailList[mailId]
    if fn then
        fn({err=err, args={...}})
    end
end)
function netcode.call(...)
    local args = {...}
    if #args < 2 then return end
    local service, fn = args[1], args[2]
    table.remove(args, 1)
    table.remove(args, 1)
    local mailId = getMailId()
    mailList[mailId] = fn
    return nc_call(mailId, service, table.unpack(args))
end
function netcode.exit()
    nc_exit()
end

function netcode.rpc_serve(...)
    return nc_rpc_serve(...)
end
function netcode.rpc_client(...)
    return nc_rpc_client(...)
end

-- kvdb
local nc_kv_open=netcode_kv_open
local nc_kv_get=netcode_kv_get
local nc_kv_set=netcode_kv_set
local nc_kv_close=netcode_kv_close

netcode_kv_open = nil
netcode_kv_get = nil
netcode_kv_set = nil
netcode_kv_close = nil

function netcode.kvdb(name)
    local errMsg = nc_kv_open(name)
    if errMsg ~= nil then return nil end

    local self = {}
    function self.get(bucket, key)
        return nc_kv_get(name, bucket, key)
    end
    function self.set(bucket, key, value)
        return nc_kv_set(name, bucket, key, value)
    end
    function self.close(bucket, key, value)
        return nc_kv_close(name)
    end
    return self
end

-- utils
local json = require 'cjson'
function json_decode(...)
    return json.decode(...)
end
function json_encode(...)
    return json.encode(...)
end
function table.tostring(data)
    local tablecache = {}
    local buffer = ""
    local padder = "    "

    local function dump(d, depth)
        local t = type(d)
        local str = tostring(d)
        if (t == "table") then
            if (tablecache[str]) then
                -- table already dumped before, so we dont
                -- dump it again, just mention it
                buffer = buffer.."<"..str..">\n"
            else
                tablecache[str] = (tablecache[str] or 0) + 1
                buffer = buffer.."("..str..") {\n"
                for k, v in pairs(d) do
                    buffer = buffer..string.rep(padder, depth + 1) .. "["..k.."] => "
                    dump(v, depth + 1)
                end
                buffer = buffer..string.rep(padder, depth) .. "}\n"
            end
        elseif (t == "number") then
            buffer = buffer.."("..t..") "..str.."\n"
        else
            buffer = buffer.."("..t..") \""..str.."\"\n"
        end
    end
    dump(data, 0)
    return buffer
end

    `
