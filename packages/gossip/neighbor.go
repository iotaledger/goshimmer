package gossip

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/network"
)

var (
	ErrNeighborQueueFull = errors.New("send queue is full")
)

const neighborQueueSize = 1000

type Neighbor struct {
	*peer.Peer
	*network.ManagedConnection

	log   *logger.Logger
	queue chan []byte

	wg             sync.WaitGroup
	closing        chan struct{}
	disconnectOnce sync.Once
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
		Peer:              peer,
		ManagedConnection: network.NewManagedConnection(conn),
		log:               log,
		queue:             make(chan []byte, neighborQueueSize),
		closing:           make(chan struct{}),
	}
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

	n.log.Infow("Connection closed",
		"read", n.BytesRead,
		"written", n.BytesWritten,
	)
	return err
}

// IsOutbound returns true if the neighbor is an outbound neighbor.
func (n *Neighbor) IsOutbound() bool {
	return GetAddress(n.Peer) == n.Conn.RemoteAddr().String()
}

func (n *Neighbor) disconnect() (err error) {
	n.disconnectOnce.Do(func() {
		close(n.closing)
		close(n.queue)
		err = n.ManagedConnection.Close()
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
			if _, err := n.ManagedConnection.Write(msg); err != nil {
				n.log.Warn("write error", "err", err)
			}
		case <-n.closing:
			return
		}
	}
}

func (n *Neighbor) readLoop() {
	defer n.wg.Done()

	// create a buffer for the packages
	b := make([]byte, maxPacketSize)

	for {
		_, err := n.ManagedConnection.Read(b)
		if nerr, ok := err.(net.Error); ok && nerr.Temporary() {
			// ignore temporary read errors.
			n.log.Debugw("temporary read error", "err", err)
			continue
		} else if err != nil {
			// return from the loop on all other errors
			if err != io.EOF && !strings.Contains(err.Error(), "use of closed network connection") {
				n.log.Warnw("read error", "err", err)
			}
			_ = n.ManagedConnection.Close()
			return
		}
	}
}

func (n *Neighbor) Write(b []byte) (int, error) {
	l := len(b)
	if l > maxPacketSize {
		n.log.Errorw("message too large", "len", l, "max", maxPacketSize)
	}

	// add to queue
	select {
	case n.queue <- b:
		return l, nil
	default:
		return 0, ErrNeighborQueueFull
	}
}
