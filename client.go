package zebou

import (
	"context"
	"crypto/tls"
	"github.com/omecodes/common/errors"
	"github.com/omecodes/common/utils/codec"
	"github.com/omecodes/common/utils/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"io"
	"sync"
	"time"
)

type Client struct {
	info         *PeerInfo
	syncMutex    sync.Mutex
	handlersLock sync.Mutex

	connectionAttempts int
	unconnectedTime    time.Time

	sendCloseSignal chan bool
	outboundStream  chan *ZeMsg
	inboundStream   chan *ZeMsg
	// messageHandlers map[string]pb.MessageHandler

	syncing       bool
	stopRequested bool

	serverAddress string
	tlsConfig     *tls.Config
	conn          *grpc.ClientConn
	client        NodesClient

	connectionStateHandler ConnectionStateHandler

	startSync chan bool
}

func (c *Client) dial() error {
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
		log.Error("zebou • could not reach the server", log.Field("at", c.serverAddress))
		return err
	}
	c.client = NewNodesClient(c.conn)
	return nil
}

func (c *Client) sync() {
	if c.isSyncing() {
		log.Info("zebou • called sync while already syncing")
		return
	}

	c.setSyncing()
	for !c.stopRequested {
		err := c.dial()
		if err != nil {
			log.Error("zebou::dial failed to reach server")
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
			log.Error("zebou • disconnected", log.Err(errors.Errorf("%d", status.Code(err))))
			log.Info("zebou • trying again...")
		}
		<-time.After(time.Second * 3)
		return
	}
	defer func() {
		if err := stream.CloseSend(); err != nil {
			log.Error("zebou • grpc stream closing error", log.Err(err))
		}
	}()

	// Here we are sure gRPC stream is open
	if c.connectionStateHandler != nil {
		c.connectionStateHandler.ConnectionState(true)
	}

	if c.connectionAttempts > 1 {
		log.Info("zebou • connected", log.Field("after", time.Since(c.unconnectedTime).String()), log.Field("attempts", c.connectionAttempts))
	} else {
		log.Info("zebou • connected")
	}
	c.connectionAttempts = 0

	wg := &sync.WaitGroup{}
	wg.Add(2)
	go c.recv(stream, wg)
	go c.send(stream, wg)
	wg.Wait()
}

func (c *Client) send(stream Nodes_SyncClient, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		log.Info("zebou • send routine done")
	}()

	for !c.stopRequested {
		select {
		case <-c.sendCloseSignal:
			log.Info("zebou • received signal to stop send routine")
			return

		case event, open := <-c.outboundStream:
			if !open {
				return
			}

			err := stream.Send(event)
			if err != nil {
				if err != io.EOF {
					log.Error("zebou • failed to send event", log.Err(err))
				}
				return
			}
		}
	}
}

func (c *Client) recv(stream Nodes_SyncClient, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		log.Info("zebou • recv routine done")
	}()

	for !c.stopRequested {
		msg, err := stream.Recv()
		if err != nil {
			c.sendCloseSignal <- true
			close(c.sendCloseSignal)
			if err != io.EOF {
				log.Error("zebou • recv message", log.Err(err))
			}
			return
		}
		c.inboundStream <- msg
		log.Info("zebou • new message", log.Field("type", msg.Type), log.Field("id", msg.Id))
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

func (c *Client) SendMsg(msg *ZeMsg) error {
	c.outboundStream <- msg
	return nil
}

func (c *Client) Send(msgType string, name string, o interface{}) error {
	encoded, err := codec.Json.Encode(o)
	if err != nil {
		return err
	}
	c.outboundStream <- &ZeMsg{
		Type:    msgType,
		Id:      name,
		Encoded: encoded,
	}
	return nil
}

func (c *Client) GetMessage() (*ZeMsg, error) {
	m, open := <-c.inboundStream
	if !open {
		return nil, io.ErrClosedPipe
	}
	return m, nil
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

func (c *Client) Connect() {
	go c.sync()
}

func NewClient(address string, config *tls.Config) *Client {
	sc := &Client{
		serverAddress:  address,
		tlsConfig:      config,
		startSync:      make(chan bool),
		outboundStream: make(chan *ZeMsg, 30),
		inboundStream:  make(chan *ZeMsg, 30),
	}
	return sc
}
