package zebou

import (
	"context"
	"github.com/google/uuid"
	"github.com/omecodes/common/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"net"
	"sync"
)

func Serve(l net.Listener, handler Handler) (*Hub, error) {
	h := &Hub{
		stopRequested: false,
		sessions:      map[string]*clientSession{},
		handler:       handler,
	}

	h.server = grpc.NewServer()
	RegisterNodesServer(h.server, h)

	go func() {
		err := h.server.Serve(l)
		if err != nil {
			log.Error("grpc::msg serve failed", log.Err(err))
		}
	}()
	return h, nil
}

type Hub struct {
	UnimplementedNodesServer
	handler       Handler
	sessionsLock  sync.Mutex
	sessions      map[string]*clientSession
	server        *grpc.Server
	stopRequested bool
}

func (s *Hub) Sync(stream Nodes_SyncServer) error {
	streamCtx := stream.Context()

	id := uuid.New().String()
	pi := &PeerInfo{ID: id}
	p, ok := peer.FromContext(streamCtx)
	if ok {
		pi.Address = p.Addr.String()
	}

	ctx := context.WithValue(context.Background(), ctxClientStream{}, stream)
	ctx = context.WithValue(ctx, ctxPeer{}, pi)

	s.handler.NewClient(ctx, pi)

	sess := handleClient(stream, func(msg *ZeMsg) {
		s.handler.OnMessage(ctx, msg)
	})
	s.saveClientSession(id, sess)

	defer s.handler.ClientQuit(context.Background(), pi)
	defer s.deleteClientSession(id)
	defer sess.Stop()

	sess.syncIn()
	return nil
}

func (s *Hub) saveClientSession(id string, session *clientSession) {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()
	s.sessions[id] = session
}

func (s *Hub) deleteClientSession(id string) {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()
	delete(s.sessions, id)
}

func (s *Hub) Broadcast(ctx context.Context, msg *ZeMsg) {
	s.sessionsLock.Lock()
	defer s.sessionsLock.Unlock()

	for id, sess := range s.sessions {
		err := sess.Send(msg)
		if err != nil {
			log.Error("broadcast: failed to send message to peer", log.Err(err), log.Field("peer", id))
		}
	}

}

func (s *Hub) Stop() error {
	s.server.Stop()
	return nil
}
