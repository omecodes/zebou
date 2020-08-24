package zebou

import (
	"context"
	"github.com/omecodes/common/errors"
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
)

type PeerInfo struct {
	ID      string
	Address string
}

type ctxPeer struct{}
type ctxClient struct{}
type ctxClientStream struct{}
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
	o := ctx.Value(ctxClientStream{})
	if o == nil {
		log.Error("Call zebou.send with wrong context. Missing client stream")
		return errors.Internal
	}
	stream := o.(pb.Nodes_SyncServer)
	return stream.Send(msg)
}

func Broadcast(ctx context.Context, msg *pb.SyncMessage) error {
	o := ctx.Value(ctxClientStream{})
	if o == nil {
		log.Error("Call zebou.send with wrong context. Missing hub")
		return errors.Internal
	}
	hub := o.(*Hub)
	hub.Broadcast(ctx, msg)
	return nil
}
