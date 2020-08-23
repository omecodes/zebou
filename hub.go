package zebou

import (
	"context"
	"github.com/google/uuid"
	"github.com/omecodes/common/utils/doer"
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
	"google.golang.org/grpc"
	"net"
	"sync"
)

func Serve(l net.Listener, handler Handler) (*Hub, error) {
	h := &Hub{
		stopRequested:      false,
		broadcastReceivers: map[string]chan *pb.SyncMessage{},
		stoppers:           map[string]doer.Stopper{},
	}

	server := grpc.NewServer()
	pb.RegisterNodesServer(server, h)

	go func() {
		err := server.Serve(l)
		if err != nil {
			log.Error("grpc::msg serve failed", log.Err(err))
		}
	}()
	return h, nil
}

type Hub struct {
	handler            Handler
	broadcastMutex     sync.Mutex
	stopMutex          sync.Mutex
	stopRequested      bool
	stoppers           map[string]doer.Stopper
	broadcastReceivers map[string]chan *pb.SyncMessage
}

func (s *Hub) Sync(stream pb.Nodes_SyncServer) error {
	broadcastReceiver := make(chan *pb.SyncMessage)
	id := s.saveBroadcastReceiver(broadcastReceiver)
	sess := handleClient(stream)
	s.saveStopper(id, sess)
	defer s.deleteBroadcastReceiver(id)
	defer s.stop(id)
	sess.sync()
	return nil
}

func (s *Hub) saveBroadcastReceiver(channel chan *pb.SyncMessage) string {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()

	id := uuid.New().String()
	s.broadcastReceivers[id] = channel
	return id
}

func (s *Hub) deleteBroadcastReceiver(key string) {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()
	c := s.broadcastReceivers[key]
	defer close(c)
	delete(s.broadcastReceivers, key)
}

func (s *Hub) Broadcast(ctx context.Context, msg *pb.SyncMessage) {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()
	for _, receiver := range s.broadcastReceivers {
		receiver <- msg
	}
}

func (s *Hub) saveStopper(id string, stopper doer.Stopper) {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	s.stoppers[id] = stopper
}

func (s *Hub) stop(id string) {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	stopper, found := s.stoppers[id]
	if found {
		err := stopper.Stop()
		log.Error("grpc::msg stopped session with error", log.Err(err))
		delete(s.stoppers, id)
	}
}

func (s *Hub) Stop() error {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	for _, stopper := range s.stoppers {
		err := stopper.Stop()
		if err != nil {
			log.Error("msg::server stop failed", log.Err(err))
		}
	}
	return nil
}
