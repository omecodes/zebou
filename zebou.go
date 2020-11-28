package zebou

import (
	"context"
)

// Sender is a convenience for message sender
type Sender interface {
	Send(message *ZeMsg) error
}

// MessageHandler is a convenience for message handler
type MessageHandler interface {
	Handle(message *ZeMsg)
}

type HandleMessageFunc func(msg *ZeMsg)

// Handler is a convenience for hub clients activity handler
type Handler interface {
	NewClient(ctx context.Context, info *PeerInfo)
	ClientQuit(ctx context.Context, info *PeerInfo)
	OnMessage(ctx context.Context, msg *ZeMsg)
}
