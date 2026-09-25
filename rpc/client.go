package rpc

import (
	"context"
	"fmt"
	"net/rpc"
	"time"
)

// Client is a typed wrapper around net/rpc for the Mamori provider protocol.
type Client struct {
	rpc *rpc.Client
}

func NewTypedClient(c *rpc.Client) *Client {
	return &Client{rpc: c}
}

func (c *Client) Close() error {
	return c.rpc.Close()
}

func (c *Client) Info(ctx context.Context) (*InfoResponse, error) {
	var reply InfoResponse
	if err := c.call(ctx, ServiceName+".Info", &InfoRequest{}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *Client) Resolve(ctx context.Context, ref string) (*ResolveResult, error) {
	req := &ResolveRequest{Ref: ref}
	if dl, ok := ctx.Deadline(); ok {
		t := dl
		req.Deadline = &t
	}
	var reply ResolveResult
	if err := c.call(ctx, ServiceName+".Resolve", req, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

func (c *Client) call(ctx context.Context, method string, args, reply any) error {
	call := c.rpc.Go(method, args, reply, nil)
	select {
	case <-ctx.Done():
		return fmt.Errorf("rpc %s: %w", method, ctx.Err())
	case <-call.Done:
		return call.Error
	}
}

// PingDeadline returns a short context for startup Info.
func PingDeadline() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 10*time.Second)
}
