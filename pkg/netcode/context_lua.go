package netcode

import (
    "context"
    "fmt"
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
    ln, err := net.Listen("tcp", addr)
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
                // defer wg.Done()
                m := &ncMail{
                    mailId:  0,
                    service: service,
                    args:    args,
                }
                reply, errBytes := c.dispatch(m)
                result = reply
                if errBytes != nil {
                    err = fmt.Errorf("%s", errBytes)
                }
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

    client, err := rpc.Dial(addr)
    if err != nil {
        L.PushString(err.Error())
    }
    c.values[service] = client
    return 0
}
