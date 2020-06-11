package zebou

import (
	"github.com/omecodes/common/doer"
	"github.com/omecodes/common/log"
	pb "github.com/omecodes/zebou/proto"
	"google.golang.org/grpc"
	"net"
	"sync"
)

func Serve(l net.Listener, messages pb.Messages) (doer.Stopper, error) {
	if messages == nil {
		messages = newMemMessageStore()
	}
	h := &server{
		messages:           messages,
		stopRequested:      false,
		broadcastReceivers: map[int]chan *pb.SyncMessage{},
		stoppers:           map[int]doer.Stopper{},
	}

	server := grpc.NewServer()
	pb.RegisterNodesServer(server, h)

	go func() {
		err := server.Serve(l)
		if err != nil {
			log.Error("grpc::msg serve failed", err)
		}
	}()
	return doer.StopFunc(h.Stop), nil
}

type server struct {
	broadcastMutex     sync.Mutex
	stopMutex          sync.Mutex
	keyCounter         int
	messages           pb.Messages
	stopRequested      bool
	stoppers           map[int]doer.Stopper
	broadcastReceivers map[int]chan *pb.SyncMessage
}

func (s *server) Sync(stream pb.Nodes_SyncServer) error {
	broadcastReceiver := make(chan *pb.SyncMessage, 30)
	sess := NewServerStreamSession(stream, broadcastReceiver, s.messages, pb.MessageHandlerFunc(s.Handle))
	id := s.saveBroadcastReceiver(broadcastReceiver)
	s.saveStopper(id, sess)

	defer s.deleteBroadcastReceiver(id)
	defer s.stop(id)
	sess.sync()
	return nil
}

func (s *server) Handle(msg *pb.SyncMessage) {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()
	for _, r := range s.broadcastReceivers {
		r <- msg
	}
	err := s.messages.Handle(msg)
	if err != nil {
		log.Error("could not save message", err)
	}
}

func (s *server) saveBroadcastReceiver(channel chan *pb.SyncMessage) int {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()
	s.keyCounter++
	s.broadcastReceivers[s.keyCounter] = channel
	return s.keyCounter
}

func (s *server) deleteBroadcastReceiver(key int) {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()
	c := s.broadcastReceivers[key]
	defer close(c)
	delete(s.broadcastReceivers, key)
}

func (s *server) broadcast(msg *pb.SyncMessage, except int) {
	s.broadcastMutex.Lock()
	defer s.broadcastMutex.Unlock()

	for id, receiver := range s.broadcastReceivers {
		if id == except {
			continue
		}
		receiver <- msg
	}
}

func (s *server) saveStopper(id int, stopper doer.Stopper) {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	s.stoppers[id] = stopper
}

func (s *server) stop(id int) {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	stopper, found := s.stoppers[id]
	if found {
		err := stopper.Stop()
		log.Error("grpc::msg stopped session with error", err)
		delete(s.stoppers, id)
	}
}

func (s *server) Stop() error {
	s.stopMutex.Lock()
	defer s.stopMutex.Unlock()
	for _, stopper := range s.stoppers {
		err := stopper.Stop()
		if err != nil {
			log.Error("msg::server stop failed", err)
		}
	}
	return nil
}

type memMessageStore struct {
	store *sync.Map
}

func (m *memMessageStore) Handle(msg *pb.SyncMessage) error {
	m.store.Store(msg.Id, msg)
	return nil
}

func (m *memMessageStore) State() ([]*pb.SyncMessage, error) {
	var list []*pb.SyncMessage

	m.store.Range(func(key, value interface{}) bool {
		list = append(list, value.(*pb.SyncMessage))
		return true
	})

	return list, nil
}

func (m *memMessageStore) Invalidate(id string) error {
	m.store.Delete(id)
	return nil
}

func newMemMessageStore() *memMessageStore {
	s := new(memMessageStore)
	s.store = &sync.Map{}
	return s
}
