package zebou

import (
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
	"io"
	"sync"
)

type clientSession struct {
	info          *PeerInfo
	hub           *Hub
	handleFunc    HandleMessageFunc
	stopRequested bool
	closed        bool
	stream        pb.Nodes_SyncServer
}

func (c *clientSession) sync() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go c.syncIn(c.stream, wg)
	wg.Wait()
}

func (c *clientSession) syncIn(stream pb.Nodes_SyncServer, wg *sync.WaitGroup) {
	defer wg.Done()

	for !c.stopRequested && !c.closed {
		msg, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Error("grpc::msg receive failed", log.Err(err))
				c.closed = true
			}
			break
		}
		c.handleFunc(msg)
	}
}

func (c *clientSession) Send(msg *pb.SyncMessage) error {
	return c.stream.SendMsg(msg)
}

func (c *clientSession) Stop() {
	c.stopRequested = true
}

func handleClient(stream pb.Nodes_SyncServer, hf HandleMessageFunc) *clientSession {
	s := &clientSession{}
	s.stream = stream
	s.stopRequested = false
	s.closed = false
	s.handleFunc = hf
	return s
}
