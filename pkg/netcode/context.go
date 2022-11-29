package netcode

import (
    "log"
    "netcode/lua"
    "netcode/pkg/utils"
    "time"
)

func Inject(L *lua.State) (*Context, error) {
    c := &Context{
        L:           L,
        values:      make(map[string]interface{}),
        services:    make(map[string]*ncService),
        serviceChan: make(chan *ncService, utils.EnvGetInt("mq_len", 1000)),
        mailChan:    make(chan *ncMail, utils.EnvGetInt("mq_len", 1000)),
        funcChan:    make(chan func(), utils.EnvGetInt("mq_len", 1000)),
        replyChan:   make(chan *ncReply, utils.EnvGetInt("mq_len", 1000)),
        closeChan:   make(chan bool),
    }
    L.RegisterFunction("netcode_start", c.start)
    L.RegisterFunction("netcode_tick", c.tick)
    L.RegisterFunction("netcode_call", c.call)
    L.RegisterFunction("netcode_reply", c.reply)
    L.RegisterFunction("netcode_exit", c.exit)
    L.RegisterFunction("netcode_rpc_serve", c.rpcServe)
    L.RegisterFunction("netcode_rpc_client", c.rpcClient)

    err := L.DoString(_context_init_script)
    if err != nil {
        return nil, err
    }
    c.tickLastTime = time.Now()
    c.tickTime()
    c.loadService()
    return c, nil
}

type (
    Context struct {
        L            *lua.State
        tickFuncRef  int
        tickLastTime time.Time
        replyFuncRef int
        services     map[string]*ncService
        serviceChan  chan *ncService
        mailChan     chan *ncMail
        replyChan    chan *ncReply
        funcChan     chan func()
        closeChan    chan bool
        values       map[string]interface{}
    }
    ncService struct {
        name string
        ref  int
        self int
    }
    ncMail struct {
        // wg      sync.WaitGroup
        mailId  int
        service string
        args    [][]byte
    }
    ncReply struct {
        mailId int
        args   [][]byte
        err    error
    }
)

func (c *Context) tickTime() {
    L := c.L

    now := time.Now()
    milli := now.UnixMilli()
    dt := now.Sub(c.tickLastTime).Milliseconds()
    c.tickLastTime = now

    // 触发tick
    L.RawGetiX(c.tickFuncRef)
    if L.IsLuaFunction(-1) {
        L.PushInteger(milli)
        L.PushInteger(dt)
        err := L.Call(2, 0)
        if err != nil {
            panic(err)
        }
    }
}
func (c *Context) loadService() {
    err := c.L.DoString(`require 'service'`)
    if err != nil {
        log.Println(err)
    }
}
func (c *Context) EnqueueAction(fn func()) {
    if len(c.mailChan) == cap(c.mailChan) {
        go func() {
            c.funcChan <- fn
        }()
        return
    }
    c.funcChan <- fn
}
func (c *Context) IncomingMail(id int, service string, args [][]byte) *ncMail {
    mail := &ncMail{
        mailId:  id,
        service: service,
        args:    args,
    }

    if len(c.mailChan) == cap(c.mailChan) { // 如果超出长度
        go func() {
            c.mailChan <- mail
        }()
        return mail
    }
    c.mailChan <- mail

    return mail
}
func (c *Context) IncomingReply(id int, payload [][]byte, err error) {
    reply := &ncReply{
        mailId: id,
        args:   payload,
        err:    err,
    }

    if len(c.replyChan) == cap(c.replyChan) { // 如果超出长度
        go func() {
            c.replyChan <- reply
        }()
        return
    }
    c.replyChan <- reply
}
func (c *Context) registerService(n *ncService) {
    if len(c.serviceChan) == cap(c.serviceChan) { // 如果超出长度
        go func() {
            c.serviceChan <- n
        }()
        return
    }
    c.serviceChan <- n
}

func (c *Context) doReply(r *ncReply) {
    L := c.L

    // 触发tick
    L.RawGetiX(c.replyFuncRef)
    if L.IsLuaFunction(-1) {
        L.PushNumber(float64(r.mailId))
        if r.err != nil {
            L.PushString(r.err.Error())
        } else {
            L.PushNil()
        }
        sz := len(r.args)
        for _, arg := range r.args {
            if arg == nil {
                L.PushNil()
            } else {
                L.PushBytes(arg)
            }
        }
        err := L.Call(sz+2, 0)
        if err != nil {
            panic(err)
        }
    }
}
