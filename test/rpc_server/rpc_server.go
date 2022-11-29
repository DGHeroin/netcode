package main

import (
    "context"
    "net"
    "netcode/pkg/rpc"
)

func main() {
    ln, err := net.Listen("tcp", ":50051")
    if err != nil {
        panic(err)
    }
    rpc.Serve(ln, func(ctx context.Context, args [][]byte) ([][]byte, error) {
        return [][]byte{[]byte("你好")}, nil
    })
}
