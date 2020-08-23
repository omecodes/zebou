package zebou

import (
	"context"
	pb "github.com/omecodes/zebou/proto"
)

type PeerInfo struct {
	ID      string
	Address string
}

type ctxPeer struct{}
type ctxClient struct{}
type ctxHub struct{}

func hub(ctx context.Context) *Hub {
	o := ctx.Value(ctxHub{})
	if o == nil {
		return nil
	}
	return o.(*Hub)
}

func client(ctx context.Context) *Client {
	o := ctx.Value(ctxClient{})
	if o == nil {
		return nil
	}
	return o.(*Client)
}

func Peer(ctx context.Context) *PeerInfo {
	o := ctx.Value(ctxPeer{})
	if o == nil {
		return nil
	}
	return o.(*PeerInfo)
}

func Send(ctx context.Context, msg *pb.SyncMessage) error {
	return nil
}

func Broadcast(ctx context.Context, msg *pb.SyncMessage) error {
	return nil
}
