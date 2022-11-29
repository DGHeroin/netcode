print('hello rpc server')
local service = '.__.rpc'
local msg = netcode.rpc_serve(service, ':50051')
print('注册服务', msg)
netcode.start(service, function()
    local self = {}
    local id = 0
    function self.on_message(action, payload)
        print(action, payload)
        id = id + 1
        if id % 2 == 1 then
            return '成功消息'
        end
        return '得!', '得!!', '得!!!'
    end
    return self
end)
