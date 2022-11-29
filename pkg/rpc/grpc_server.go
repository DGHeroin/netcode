package rpc

import (
    "context"
    "google.golang.org/grpc"
    "google.golang.org/grpc/reflection"
    "net"
    "netcode/pkg/rpc/pb"
)

type server struct {
    fn func(context.Context, [][]byte) ([][]byte, error)
}

func (s *server) Request(ctx context.Context, in *pb.Message) (*pb.Message, error) {
    b, err := s.fn(ctx, in.Args)
    if err != nil {
        return nil, err
    }
    return &pb.Message{Args: b}, nil
}

func Serve(ln net.Listener, fn func(ctx context.Context, args [][]byte) ([][]byte, error)) error {
    s := grpc.NewServer()
    pb.RegisterRServiceServer(s, &server{fn: fn})
    reflection.Register(s)
    return s.Serve(ln)
}
