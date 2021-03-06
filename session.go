package zebou

import (
	"github.com/omecodes/common/utils/log"
	"io"
)

type clientSession struct {
	info          *PeerInfo
	hub           *Hub
	handleFunc    HandleMessageFunc
	stopRequested bool
	closed        bool
	stream        Nodes_SyncServer
}

func (c *clientSession) syncIn() {
	for !c.stopRequested && !c.closed {
		msg, err := c.stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Error("zebou session •  receive failed", log.Err(err))
				c.closed = true
			}
			return
		}
		go c.handleFunc(msg)
	}
}

func (c *clientSession) Send(msg *ZeMsg) error {
	return c.stream.SendMsg(msg)
}

func (c *clientSession) Stop() {
	c.stopRequested = true
}

func handleClient(stream Nodes_SyncServer, hf HandleMessageFunc) *clientSession {
	s := &clientSession{}
	s.stream = stream
	s.stopRequested = false
	s.closed = false
	s.handleFunc = hf
	return s
}
