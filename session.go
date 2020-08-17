package zebou

import (
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
	"io"
	"sync"
	"time"
)

type ServerStreamSession struct {
	msgHandler       pb.MessageHandler
	messages         pb.Messages
	broadcastChannel chan *pb.SyncMessage
	stopRequested    bool
	closed           bool
	stream           pb.Nodes_SyncServer
}

func (s *ServerStreamSession) sync() {
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go s.syncIn(s.stream, wg)
	go s.syncOut(s.stream, wg)
	wg.Wait()
}

func (s *ServerStreamSession) syncIn(stream pb.Nodes_SyncServer, wg *sync.WaitGroup) {
	defer wg.Done()
	var registeredMessages []string

	for !s.stopRequested && !s.closed {
		msg, err := stream.Recv()
		if err != nil {
			if err != io.EOF {
				log.Error("grpc::msg receive failed", log.Err(err))
				s.closed = true
			}
			break
		}

		err = s.messages.Handle(msg)
		if err != nil {
			log.Error("grpc::msg storing failed", log.Err(err))
			s.closed = true
			break
		}

		go s.msgHandler.Handle(msg)

		registeredMessages = append(registeredMessages, msg.Id)
	}

	log.Info("grpc::msg closing inbound traffic")
	if !s.stopRequested {
		for _, key := range registeredMessages {
			err := s.messages.Invalidate(key)
			if err != nil {
				log.Error("grpc::msg could not invalidate object", log.Err(err))
			}
		}
	}
}

func (s *ServerStreamSession) syncOut(stream pb.Nodes_SyncServer, wg *sync.WaitGroup) {
	defer wg.Done()
	<-time.After(time.Second)

	list, err := s.messages.State()
	if err != nil {
		log.Error("grpc::msg sending messages", log.Err(err))
		return
	}

	log.Info("grpc::msg sending all messages from store")
	for _, o := range list {
		err = stream.SendMsg(o)
		if err != nil {
			log.Error("grpc::msg send event", log.Err(err))
			s.closed = true
			return
		}
	}

	for !s.stopRequested && !s.closed {
		o, open := <-s.broadcastChannel
		if !open {
			return
		}

		err := stream.SendMsg(o)
		if err != nil {
			log.Error("grpc::msg send event", log.Err(err))
		}
	}
}

func (s *ServerStreamSession) Stop() error {
	s.stopRequested = true
	return nil
}

func NewServerStreamSession(stream pb.Nodes_SyncServer, broadcastChannel chan *pb.SyncMessage, messages pb.Messages, handler pb.MessageHandler) *ServerStreamSession {
	s := &ServerStreamSession{}
	s.stream = stream
	s.messages = messages
	s.broadcastChannel = broadcastChannel
	s.stopRequested = false
	s.closed = false
	s.msgHandler = handler
	return s
}
