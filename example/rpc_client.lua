local client = netcode.rpc_client('login-watch-dog', 'localhost:50051')
print('启动rpc', client)

local resp, err = netcode.call('login-watch-dog',  function(cmd)
    print('请求:', table.tostring(cmd))
end, 'hello', 'body-payload-yes')
