package gossip

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/netutil"
	"github.com/iotaledger/hive.go/netutil/buffconn"
	"go.uber.org/atomic"
)

const (
	neighborQueueSize        = 5000
	maxNumReadErrors         = 10
	droppedMessagesThreshold = 1000
)

// Neighbor describes the established gossip connection to another peer.
type Neighbor struct {
	*peer.Peer
	*buffconn.BufferedConnection

	log             *logger.Logger
	queue           chan []byte
	messagesDropped atomic.Int32

	wg             sync.WaitGroup
	closing        chan struct{}
	disconnectOnce sync.Once

	connectionEstablished time.Time
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(peer *peer.Peer, conn net.Conn, log *logger.Logger) *Neighbor {
	if !IsSupported(peer) {
		panic("peer does not support gossip")
	}

	// always include ID and address with every log message
	log = log.With(
		"id", peer.ID(),
		"network", conn.LocalAddr().Network(),
		"addr", conn.RemoteAddr().String(),
	)

	return &Neighbor{
		Peer:                  peer,
		BufferedConnection:    buffconn.NewBufferedConnection(conn, maxPacketSize),
		log:                   log,
		queue:                 make(chan []byte, neighborQueueSize),
		closing:               make(chan struct{}),
		connectionEstablished: time.Now(),
	}
}

// ConnectionEstablished returns the connection established.
func (n *Neighbor) ConnectionEstablished() time.Time {
	return n.connectionEstablished
}

// Listen starts the communication to the neighbor.
func (n *Neighbor) Listen() {
	n.wg.Add(2)
	go n.readLoop()
	go n.writeLoop()

	n.log.Info("Connection established")
}

// Close closes the connection to the neighbor and stops all communication.
func (n *Neighbor) Close() error {
	err := n.disconnect()
	// wait for everything to finish
	n.wg.Wait()

	n.log.Info("Connection closed")
	return err
}

// IsOutbound returns true if the neighbor is an outbound neighbor.
func (n *Neighbor) IsOutbound() bool {
	return GetAddress(n.Peer) == n.RemoteAddr().String()
}

func (n *Neighbor) disconnect() (err error) {
	n.disconnectOnce.Do(func() {
		close(n.closing)
		err = n.BufferedConnection.Close()
	})
	return
}

func (n *Neighbor) writeLoop() {
	defer n.wg.Done()

	for {
		select {
		case msg := <-n.queue:
			if len(msg) == 0 {
				continue
			}
			if _, err := n.BufferedConnection.Write(msg); err != nil {
				n.log.Warnw("Write error", "err", err)
				_ = n.BufferedConnection.Close()
				return
			}
		case <-n.closing:
			return
		}
	}
}

func (n *Neighbor) readLoop() {
	defer n.wg.Done()

	var numReadErrors uint
	for {
		err := n.Read()
		if netutil.IsTemporaryError(err) {
			// ignore temporary read errors.
			n.log.Debugw("temporary read error", "err", err)
			numReadErrors++
			if numReadErrors > maxNumReadErrors {
				n.log.Warnw("Too many read errors", "err", err)
				_ = n.BufferedConnection.Close()
				return
			}
			continue
		}
		if err != nil {
			// return from the loop on all other errors
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				n.log.Warnw("Permanent error", "err", err)
			}
			_ = n.BufferedConnection.Close()
			return
		}
	}
}

func (n *Neighbor) Write(b []byte) (int, error) {
	l := len(b)
	if l > maxPacketSize {
		n.log.Panicw("message too large", "len", l, "max", maxPacketSize)
	}

	// add to queue
	select {
	case n.queue <- b:
		return l, nil
	case <-n.closing:
		return 0, nil
	default:
		if n.messagesDropped.Inc() >= droppedMessagesThreshold {
			n.messagesDropped.Store(0)
			return 0, ErrNeighborQueueFull
		}
		return 0, nil
	}
}
