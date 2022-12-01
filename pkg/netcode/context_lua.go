package netcode

import (
    "context"
    "crypto/tls"
    "net"
    "netcode/lua"
    "netcode/pkg/rpc"
    "sync"
)

func (c *Context) start(L *lua.State) int {
    // 先注册到列表, 下一个tick再调用
    name := L.ToString(-2)
    if L.IsLuaFunction(-1) {
        c.registerService(&ncService{name: name, ref: L.RefX()})
    }
    return 0
}
func (c *Context) tick(L *lua.State) int {
    if L.IsLuaFunction(-1) {
        c.tickFuncRef = L.RefX()
    }
    return 1
}
func (c *Context) reply(L *lua.State) int {
    if L.IsLuaFunction(-1) {
        c.replyFuncRef = L.RefX()
    }
    return 1
}
func (c *Context) call(L *lua.State) int {
    if !L.IsNumber(2) { // 邮件id
        return 0
    }
    if !L.IsString(3) { // 服务名称
        return 0
    }

    mailId := L.ToInteger(2)
    service := L.ToString(3)
    n := L.GetTop()
    var args [][]byte
    for i := 4; i <= n; i++ {
        args = append(args, L.ToBytes(i))
    }

    c.IncomingMail(mailId, service, args)

    return 0
}
func (c *Context) exit(*lua.State) int {
    close(c.closeChan)
    return 0
}
func (c *Context) rpcServe(L *lua.State) int {
    if !L.IsString(2) {
        L.PushString("args need name")
        return 1
    }
    if !L.IsString(3) {
        L.PushString("args need address")
        return 1
    }

    service := L.ToString(2)
    addr := L.ToString(3)

    var (
        ln  net.Listener
        err error
    )

    if L.IsString(4) && L.IsString(5) { // cert, key
        crt, err := tls.LoadX509KeyPair(L.ToString(4), L.ToString(5))
        if err != nil {
            L.PushString(err.Error())
            return 1
        }
        tlsConfig := &tls.Config{
            InsecureSkipVerify: true,
        }
        tlsConfig.Certificates = []tls.Certificate{crt}
        ln, err = tls.Listen("tcp", addr, tlsConfig)
    } else {
        // 普通
        ln, err = net.Listen("tcp", addr)
    }
    if err != nil {
        L.PushString(err.Error())
        return 1
    }

    go func() {
        var err error
        defer func() {
            if e := recover(); e != nil {
            }
            err = ln.Close()
            if err != nil {
            }
        }()
        err = rpc.Serve(ln, func(ctx context.Context, args [][]byte) (result [][]byte, err error) {
            var wg sync.WaitGroup
            wg.Add(1)
            c.EnqueueAction(func() {
                defer wg.Done()
                m := &ncMail{
                    service: service,
                    args:    args,
                }
                result, err = c.dispatch(m)
            })
            wg.Wait()
            return
        })

    }()
    return 0
}
func (c *Context) rpcClient(L *lua.State) int {
    if !L.IsString(2) {
        L.PushString("args need name")
        return 1
    }
    if !L.IsString(3) {
        L.PushString("args need address")
        return 1
    }

    service := L.ToString(2)
    addr := L.ToString(3)
    var (
        client rpc.Client
        err    error
    )
    if L.IsString(4) && L.IsString(5) { // cert, key
        crt, err := tls.LoadX509KeyPair(L.ToString(4), L.ToString(5))
        if err != nil {
            L.PushString(err.Error())
            return 1
        }
        tlsConfig := &tls.Config{
            InsecureSkipVerify: true,
        }
        tlsConfig.Certificates = []tls.Certificate{crt}
        client, err = rpc.DialTLS(addr, tlsConfig)
    } else {
        client, err = rpc.Dial(addr)
    }

    if err != nil {
        L.PushString(err.Error())
        return 1
    }
    c.values[service] = client
    return 0
}

func (c *Context) storageOpen(L *lua.State) int {
    if !L.IsString(2) {
        L.PushString("args need name")
        return 1
    }
    dbName := L.ToString(2)
    c.values["__db_"+dbName] = c.StorageOpen(dbName)
    return 0
}
func (c *Context) storageGet(L *lua.State) int {
    if !L.IsString(2) || !L.IsString(3) || !L.IsString(4) {
        L.PushString("args invalid")
        L.PushBoolean(false)
        return 2
    }

    dbName := L.ToString(2)
    bucket := L.ToString(3)
    key := L.ToString(4)
    p, ok := c.values["__db_"+dbName]
    if !ok {
        L.PushString("db not found")
        L.PushBoolean(false)
        return 2
    }
    val, err := c.StorageGet(p, bucket, key)
    if err != nil {
        L.PushString(err.Error())
        L.PushBoolean(false)
        return 2
    }
    L.PushBytes(val)
    L.PushBoolean(true)

    return 2
}
func (c *Context) storageSet(L *lua.State) int {
    if !L.IsString(2) || !L.IsString(3) || !L.IsString(4) || !L.IsString(5) {
        L.PushString("args invalid")
        return 1
    }
    dbName := L.ToString(2)
    bucket := L.ToString(3)
    key := L.ToString(4)
    val := L.ToBytes(5)
    p, ok := c.values["__db_"+dbName]
    if !ok {
        L.PushString("db not found")
        return 1
    }

    err := c.StorageSet(p, bucket, key, val)
    if err != nil {
        L.PushString(err.Error())
        return 1
    }
    return 0
}

func (c *Context) storageClose(L *lua.State) int {
    if !L.IsString(2) {
        L.PushString("args invalid")
        return 1
    }
    dbName := L.ToString(2)

    p, ok := c.values["__db_"+dbName]
    if !ok {
        L.PushString("db not found")
        return 1
    }
    err := c.StorageClose(p)
    if err != nil {
        L.PushString(err.Error())
        return 1
    }
    delete(c.values, "__db_"+dbName)
    return 0
}
