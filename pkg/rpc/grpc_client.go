package rpc

import (
    "context"
    "crypto/tls"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials"
    "google.golang.org/grpc/credentials/insecure"
    "netcode/pkg/rpc/pb"
)

type Client interface {
    Close() error
    Request(ctx context.Context, args ...[]byte) ([][]byte, error)
}
type client struct {
    cli  pb.RServiceClient
    conn *grpc.ClientConn
}

func (c *client) Close() error {
    return c.conn.Close()
}

func (c *client) Request(ctx context.Context, args ...[]byte) ([][]byte, error) {
    var (
        req = pb.Message{
            Args: args,
        }
    )
    resp, err := c.cli.Request(ctx, &req)
    if err != nil {
        return nil, err
    }
    return resp.Args, nil
}

func Dial(address string) (Client, error) {
    conn, err := grpc.Dial(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
    if err != nil {
        return nil, err
    }
    c := pb.NewRServiceClient(conn)
    return &client{cli: c, conn: conn}, nil
}
func DialTLS(address string, config *tls.Config) (Client, error) {
    conn, err := grpc.Dial(address, grpc.WithTransportCredentials(credentials.NewTLS(config)))
    if err != nil {
        return nil, err
    }
    c := pb.NewRServiceClient(conn)
    return &client{cli: c, conn: conn}, nil
}
