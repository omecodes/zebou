package pb

type MessageHandlerFunc func(*SyncMessage)

func (h MessageHandlerFunc) Handle(msg *SyncMessage) {
	h(msg)
}

type MessageHandler interface {
	Handle(*SyncMessage)
}

type Messages interface {
	Handle(*SyncMessage) error
	State() ([]*SyncMessage, error)
	Invalidate(string) error
}
