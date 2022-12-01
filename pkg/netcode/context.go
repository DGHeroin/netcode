package netcode

import (
    "container/list"
    bolt "go.etcd.io/bbolt"
    "log"
    "netcode/lua"
    "netcode/pkg/utils"
    "sync"
    "time"
)

func Inject(L *lua.State) (*Context, error) {
    mqSize := utils.EnvGetInt("mq_len", 5000)
    c := &Context{
        L:           L,
        values:      make(map[string]interface{}),
        services:    make(map[string]*ncService),
        serviceChan: make(chan *ncService, mqSize),
        mails:       list.New(),
        funcChan:    make(chan func(), mqSize),
        closeChan:   make(chan bool),
    }
    L.RegisterFunction("netcode_start", c.start)
    L.RegisterFunction("netcode_tick", c.tick)
    L.RegisterFunction("netcode_call", c.call)
    L.RegisterFunction("netcode_reply", c.reply)
    L.RegisterFunction("netcode_exit", c.exit)
    L.RegisterFunction("netcode_rpc_serve", c.rpcServe)
    L.RegisterFunction("netcode_rpc_client", c.rpcClient)
    L.RegisterFunction("netcode_kv_open", c.storageOpen)
    L.RegisterFunction("netcode_kv_get", c.storageGet)
    L.RegisterFunction("netcode_kv_set", c.storageSet)
    L.RegisterFunction("netcode_kv_close", c.storageClose)

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
        mailsMu      sync.Mutex
        mails        *list.List
        serviceChan  chan *ncService
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
    if len(c.funcChan) == cap(c.funcChan) {
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
    c.mailsMu.Lock()
    defer c.mailsMu.Unlock()

    c.mails.PushBack(mail)

    return mail
}
func (c *Context) IncomingReply(id int, payload [][]byte, err error) {
    reply := &ncReply{
        mailId: id,
        args:   payload,
        err:    err,
    }
    c.doReply(reply)
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

        }
    }
}

func (c *Context) tickMail(mails *list.List) {
    for it := mails.Front(); it != nil; it = it.Next() {
        m := it.Value.(*ncMail)
        payload, err := c.dispatch(m)
        c.IncomingReply(m.mailId, payload, err)
    }
}

func (c *Context) release() {
    for _, p := range c.values {
        switch obj := p.(type) {
        case *bolt.DB:
            _ = obj.Close()
        }
    }
}
