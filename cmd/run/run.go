package run

import (
    "errors"
    "github.com/spf13/cobra"
    "io/ioutil"
    "netcode/lua"
    "netcode/pkg/netcode"
    "os"
    "os/signal"
    "syscall"
)

var (
    Cmd = &cobra.Command{
        Use: "run <filename>",
        RunE: func(cmd *cobra.Command, args []string) error {
            if len(args) != 1 {
                return errors.New("need input file")
            }
            filename := args[0]
            return runFile(filename)
        },
    }
)

func runFile(filename string) error {
    L := lua.NewState()
    defer L.Close()
    L.OpenLibs()
    L.OpenLibsExt()
    var (
        err error
        ctx *netcode.Context
    )
    ctx, err = netcode.Inject(L)
    if err != nil {
        return err
    }
    if ctx != nil {
    }
    ctx.EnqueueAction(func() {
        var data []byte
        data, err = ioutil.ReadFile(filename)
        if err != nil {
            ctx.Close()
            return
        }
        err = L.DoString(string(data))
        if err != nil {
            ctx.Close()
            return
        }
    })

    {
        go func() {
            sigs := make(chan os.Signal, 1)
            signal.Notify(sigs, syscall.SIGINT,
                syscall.SIGILL,
                syscall.SIGFPE,
                syscall.SIGSEGV,
                syscall.SIGTERM,
                syscall.SIGABRT, )
            <-sigs
            ctx.Close()
        }()
    }

    ctx.Loop()
    return err
}
