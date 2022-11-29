package main

import (
    "context"
    "fmt"
    "netcode/pkg/rpc"
    "time"
)

func main() {
    const address = "localhost:50051"
    c, err := rpc.Dial(address)
    if err != nil {
        panic(err)
    }

    for {
        sendReq(c)
        time.Sleep(time.Second)
    }
}
func sendReq(c rpc.Client) {
    rsp, err := c.Request(context.Background(), []byte("login"), []byte("login-info"))
    if err != nil {
        fmt.Println(err)
        return
    }
    fmt.Println(rsp)
}
