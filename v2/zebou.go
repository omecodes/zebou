package zebou

import (
	"context"
)

type Sender interface {
	Send(message *ZeMsg) error
}

type MessageHandler interface {
	Handle(message *ZeMsg)
}

type HandleMessageFunc func(msg *ZeMsg)

type Handler interface {
	NewClient(ctx context.Context, info *PeerInfo)
	ClientQuit(ctx context.Context, info *PeerInfo)
	OnMessage(ctx context.Context, msg *ZeMsg)
}
