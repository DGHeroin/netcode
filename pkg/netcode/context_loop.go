package netcode

import (
    "context"
    "fmt"
    "log"
    "netcode/pkg/rpc"
    "os"
    "strconv"
    "time"
)

func (c *Context) Loop() {
    tickPeriod := time.Second
    if str := os.Getenv("tick_period"); str != "" {
        if val, err := strconv.Atoi(str); err == nil {
            tickPeriod = time.Millisecond * time.Duration(val)
        }
    }
    ticker := time.NewTicker(tickPeriod)
    for {
        select {
        case <-ticker.C: // 按帧率tick
            c.tickTime()
        case v := <-c.serviceChan:
            c.boot(v)
        case fn := <-c.funcChan:
            fn()
        case m := <-c.mailChan:
            payload, err := c.dispatch(m)
            c.IncomingReply(m.mailId, payload, err)
        case r := <-c.replyChan:
            c.doReply(r)
        case <-c.closeChan:
            return
        }
    }
}
func (c *Context) Close() {
    close(c.closeChan)
}
func (c *Context) boot(v *ncService) {
    L := c.L
    L.RawGetiX(v.ref)
    if L.IsLuaFunction(-1) {
        if err := L.Call(0, 1); err != nil {
            st := L.StackTrace()
            s := st[len(st)-1]
            log.Printf("启动服务%v错误:error: %v +%v %v\n", v.name, s.Source, s.CurrentLine, err)
        }
        if L.Type(-1) == 5 && v.name != "" { // 有返回值
            v.self = L.RefX()
            c.services[v.name] = v
        }
        L.UnrefX(v.ref)
    }
}
func (c *Context) dispatch(m *ncMail) (replyPayload [][]byte, err error) {
    L := c.L
    if service, ok := c.services[m.service]; ok {
        L.SetTop(0)
        L.RawGetiX(service.self)
        if L.Type(-1) == 5 { // table
            L.GetField(-1, "on_message")
            if L.IsLuaFunction(-1) {
                sz := len(m.args)
                for _, arg := range m.args {
                    if arg == nil {
                        L.PushNil()
                    } else {
                        L.PushBytes(arg)
                    }
                }
                err = L.Call(sz, -1)
                for i := 2; i <= L.GetTop(); i++ {
                    if L.IsString(i) {
                        replyPayload = append(replyPayload, L.ToBytes(i))
                    } else {
                        replyPayload = append(replyPayload, nil)
                    }
                }
            } else {
                err = fmt.Errorf("on_message not found")
            }
        } else {
            err = fmt.Errorf("service ref loss")
        }
    } else {
        if p, ok := c.values[m.service]; ok {
            switch v := p.(type) {
            case rpc.Client:
                ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
                defer cancel()
                replyPayload, err = v.Request(ctx, m.args...)
                return
            }
        }
        err = fmt.Errorf("service not found")
    }
    return
}
