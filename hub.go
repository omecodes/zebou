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
	sessionsMutex sync.Mutex
	sessions      map[string]*clientSession
	server        *grpc.Server
	err           error
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
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	s.sessions[id] = session
}

func (s *Hub) getClientSession(id string) (*clientSession, error) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	sess, found := s.sessions[id]
	if !found {
		return nil, SessionNotFound
	}
	return sess, nil
}

func (s *Hub) deleteClientSession(id string) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	delete(s.sessions, id)
}

// Broadcast dispatches msg
func (s *Hub) Broadcast(ctx context.Context, msg *ZeMsg) {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()

	for id, sess := range s.sessions {
		err := sess.Send(msg)
		if err != nil {
			log.Error("broadcast: failed to send message to peer", log.Err(err), log.Field("peer", id))
		}
	}
}

// SendTo sends a message to the client associated with id
func (s *Hub) SendTo(id string, msg *ZeMsg) error {
	session, err := s.getClientSession(id)
	if err != nil {
		return err
	}
	return session.Send(msg)
}

// Send sends a message to the client associated with context
// It loads peer info in context to identify client
func (s *Hub) Send(ctx context.Context, msg *ZeMsg) error {
	p := Peer(ctx)
	if p == nil {
		return NoPeerFromContext
	}
	return s.SendTo(p.ID, msg)
}

// ClientSendChannelFromContext returns the client session associated with peer info in context
func (s *Hub) ClientSendChannelFromContext(ctx context.Context) (Sender, error) {
	p := Peer(ctx)
	if p == nil {
		return nil, NoPeerFromContext
	}
	return s.getClientSession(p.ID)
}

// GetClientSendChannel returns the client session associated with id
func (s *Hub) GetClientSendChannel(id string) (Sender, error) {
	return s.getClientSession(id)
}

// Stop stops the listening for connections and free all current sessions
func (s *Hub) Stop() error {
	s.server.Stop()
	s.stopSessions()
	return nil
}

func (s *Hub) stopSessions() {
	s.sessionsMutex.Lock()
	defer s.sessionsMutex.Unlock()
	for _, s := range s.sessions {
		s.Stop()
	}
}
