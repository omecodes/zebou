package zebou

import (
	"context"
	"crypto/tls"
	"github.com/omecodes/common/errors"
	"github.com/omecodes/common/utils/codec"
	"github.com/omecodes/common/utils/log"
	pb "github.com/omecodes/zebou/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"io"
	"sync"
	"time"
)

type Client struct {
	syncMutex    sync.Mutex
	handlersLock sync.Mutex

	connectionAttempts int
	unconnectedTime    time.Time

	sendCloseSignal chan bool
	outboundStream  chan *pb.SyncMessage
	// messageHandlers map[string]pb.MessageHandler

	msgHandler pb.MessageHandler

	syncing       bool
	stopRequested bool

	serverAddress string
	tlsConfig     *tls.Config
	conn          *grpc.ClientConn
	client        pb.NodesClient

	connectionStateHandler ConnectionStateHandler

	startSync chan bool
}

func (c *Client) connect() error {
	if c.conn != nil && c.conn.GetState() == connectivity.Ready {
		return nil
	}

	var opts []grpc.DialOption
	if c.tlsConfig != nil {
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(c.tlsConfig)))
	} else {
		opts = append(opts, grpc.WithInsecure())
	}

	var err error
	c.conn, err = grpc.Dial(c.serverAddress, opts...)
	if err != nil {
		return err
	}
	c.client = pb.NewNodesClient(c.conn)
	return nil
}

func (c *Client) sync() {
	if c.isSyncing() {
		return
	}
	c.setSyncing()
	for !c.stopRequested {
		err := c.connect()
		if err != nil {
			time.After(time.Second * 2)
			continue
		}
		c.work()
		if c.connectionStateHandler != nil {
			c.connectionStateHandler.ConnectionState(false)
		}
	}
}

func (c *Client) work() {
	c.sendCloseSignal = make(chan bool)

	c.connectionAttempts++

	stream, err := c.client.Sync(context.Background())
	if err != nil {
		c.conn = nil
		if c.connectionAttempts == 1 {
			c.unconnectedTime = time.Now()
			log.Error("grpc::msg disconnected", errors.Errorf("%d", status.Code(err)))
			log.Info("grpc::msg trying again...")
		}
		<-time.After(time.Second * 3)
		return
	}
	defer stream.CloseSend()

	if c.connectionStateHandler != nil {
		c.connectionStateHandler.ConnectionState(true)
	}

	if c.connectionAttempts > 1 {
		log.Info("grpc::msg connected", log.Field("after", time.Since(c.unconnectedTime).String()), log.Field("attempts", c.connectionAttempts))
	} else {
		log.Info("grpc::msg connected")
	}
	c.connectionAttempts = 0

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go c.recv(stream, wg)
	go c.send(stream, wg)
	wg.Wait()
}

func (c *Client) send(stream pb.Nodes_SyncClient, wg *sync.WaitGroup) {
	defer wg.Done()

	for !c.stopRequested {
		select {
		case <-c.sendCloseSignal:
			log.Info("grpc::msg stop send")
			return

		case event, open := <-c.outboundStream:
			if !open {
				return
			}

			err := stream.Send(event)
			if err != nil {
				if err != io.EOF {
					log.Error("grpc::msg send event", err)
				}
				return
			}
		}
	}
}

func (c *Client) recv(stream pb.Nodes_SyncClient, wg *sync.WaitGroup) {
	defer wg.Done()
	for !c.stopRequested {
		msg, err := stream.Recv()
		if err != nil {
			c.sendCloseSignal <- true
			close(c.sendCloseSignal)
			if err != io.EOF {
				log.Error("grpc::msg recv event", err)
			}
			return
		}

		if c.msgHandler != nil {
			go c.msgHandler.Handle(msg)
		}

		log.Info("grpc::msg new event", log.Field("type", msg.Type), log.Field("id", msg.Id))
	}
}

func (c *Client) isSyncing() bool {
	c.syncMutex.Lock()
	defer c.syncMutex.Unlock()
	return c.syncing
}

func (c *Client) setSyncing() {
	c.syncMutex.Lock()
	defer c.syncMutex.Unlock()
	c.syncing = true
}

func (c *Client) Send(msgType string, name string, o interface{}) error {
	encoded, err := codec.Json.Encode(o)
	if err != nil {
		return err
	}
	c.outboundStream <- &pb.SyncMessage{
		Type:    msgType,
		Id:      name,
		Encoded: encoded,
	}
	return nil
}

func (c *Client) SendMsg(msg *pb.SyncMessage) error {
	c.outboundStream <- msg
	return nil
}

func (c *Client) Stop() error {
	c.stopRequested = true
	if c.conn != nil {
		return c.conn.Close()
	}
	close(c.outboundStream)
	return nil
}

func (c *Client) SetConnectionSateHandler(h ConnectionStateHandler) {
	c.connectionStateHandler = h
}

func Connect(address string, config *tls.Config, handler pb.MessageHandler) *Client {
	sc := &Client{
		serverAddress:  address,
		msgHandler:     handler,
		tlsConfig:      config,
		startSync:      make(chan bool),
		outboundStream: make(chan *pb.SyncMessage, 30),
	}
	go sc.sync()
	return sc
}
