package zebou

import (
	"context"
	pb "github.com/omecodes/zebou/proto"
)

type Sender interface {
	Send(message *pb.SyncMessage) error
}

type Handler interface {
	NewClient(ctx context.Context, info *PeerInfo)
	ClientQuit(ctx context.Context, info *PeerInfo)
	OnMessage(ctx context.Context, msg *pb.SyncMessage)
}
