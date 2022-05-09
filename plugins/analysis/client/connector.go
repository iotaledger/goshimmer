package client

import (
	"net"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/netutil"
	"github.com/iotaledger/hive.go/network"
)

// errors returned by the Connector
var (
	errNoConnection = errors.New("no connection established")
)

// time after which a failed Dial is retried
var redialInterval = 1 * time.Minute

// A Connector is a redialing Writer on the underlying connection.
type Connector struct {
	network string
	address string

	mu   sync.Mutex
	conn *network.ManagedConnection

	startOnce sync.Once
	stopOnce  sync.Once
	stopping  chan struct{}
}

// NewConnector creates a new Connector.
func NewConnector(network string, address string) *Connector {
	return &Connector{
		network:  network,
		address:  address,
		stopping: make(chan struct{}),
	}
}

// Start starts the Connector.
func (c *Connector) Start() {
	c.startOnce.Do(c.dial)
}

// Stop stops the Connector.
func (c *Connector) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopping)
		_ = c.Close()
	})
}

// Close closes the current connection.
// If the Connector is not closed, a new redial will be triggered.
func (c *Connector) Close() (err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err = c.conn.Close()
	}
	return
}

// Write writes data to the underlying connection.
// It returns an error if currently no connection has been established.
func (c *Connector) Write(b []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return 0, errNoConnection
	}
	n, err := c.conn.Write(b)
	// TODO: should the closing rather only happen outside?
	// TODO: is the IsTemporaryError useful here?
	if err != nil && !netutil.IsTemporaryError(err) {
		_ = c.conn.Close()
	}
	return n, err
}

func (c *Connector) dial() {
	c.mu.Lock()
	defer c.mu.Unlock()

	select {
	case <-c.stopping:
		return
	default:
		c.conn = nil
		conn, err := net.DialTimeout(c.network, c.address, 5*time.Second)
		if err != nil {
			go c.scheduleRedial()
			return
		}
		c.conn = network.NewManagedConnection(conn)
		c.conn.Events.Close.Attach(event.NewClosure(func(event *network.CloseEvent) {
			c.dial()
		}))
	}
}

func (c *Connector) scheduleRedial() {
	t := time.NewTimer(redialInterval)
	defer t.Stop()
	select {
	case <-c.stopping:
		return
	case <-t.C:
		c.dial()
	}
}
