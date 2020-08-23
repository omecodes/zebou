package zebou

import (
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
	"io"
	"sync"
)

type clientHandler struct {
	info          *PeerInfo
	hub           *Hub
	handleFunc    HandleMessageFunc
	stopRequested bool
	closed        bool
	stream        pb.Nodes_SyncServer
}

func (c *clientHandler) sync() {
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go c.syncIn(c.stream, wg)
	wg.Wait()
}

func (c *clientHandler) syncIn(stream pb.Nodes_SyncServer, wg *sync.WaitGroup) {
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

func (c *clientHandler) Send(msg *pb.SyncMessage) error {
	return c.stream.SendMsg(msg)
}

func (c *clientHandler) Stop() error {
	c.stopRequested = true
	return nil
}

func handleClient(stream pb.Nodes_SyncServer, broadcastChannel chan *pb.SyncMessage, hf HandleMessageFunc) *clientHandler {
	s := &clientHandler{}
	s.stream = stream
	s.stopRequested = false
	s.closed = false
	s.handleFunc = hf
	return s
}
