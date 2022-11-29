netcode.start('main', function()
    netcode.log('first init:', os.time(), netcode.frameCount)
--     netcode.on_tick(function()
--         print(netcode.time, netcode.deltaTime, netcode.frameCount)
--     end)

--     local updater = netcode.add_updater()
--     updater.add(function()
--        print('updater', netcode.time, netcode.frameCount)
--     end)

    netcode.call('login-service', function(cmd)
        print('登录', table.tostring(cmd))
    end, 'hello', 'payload')

    netcode.call('login-service',function(cmd)
        print('乒乓', table.tostring(cmd))
    end, 'ping-pong', 'yes yes yes', 'ccc')
    netcode.call('login-service',function(cmd)
        print('再见', table.tostring(cmd))
--         netcode.exit()
    end, 'bye')

    local self = {}
    function self.on_message(msg)

    end
    return self
end)
