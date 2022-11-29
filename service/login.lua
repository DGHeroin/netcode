netcode.start('login-service', function()
    print('start login service',  netcode.frameCount)
    local self = {}
    function self.on_message( ... )
        local args = {...}
        print('收到消息', table.tostring(args))
        if args[1] == 'bye' then
            return  'i am error!', '1', '22', '333'
        end

        return '你好'
    end
    return self
end)