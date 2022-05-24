package gossip

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/mux"
	"github.com/libp2p/go-yamux/v2"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
)

// NeighborsGroup is an enum type for various neighbors groups like auto/manual.
type NeighborsGroup int8

const (
	// NeighborsGroupAuto represents a neighbors group that is managed automatically.
	NeighborsGroupAuto NeighborsGroup = iota
	// NeighborsGroupManual represents a neighbors group that is managed manually.
	NeighborsGroupManual
)

// Neighbor describes the established gossip connection to another peer.
type Neighbor struct {
	*peer.Peer
	Group NeighborsGroup

	log            *logger.Logger
	disconnectOnce sync.Once
	wg             sync.WaitGroup

	disconnected   *events.Event
	packetReceived *events.Event

	ps *packetsStream
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, ps *packetsStream, log *logger.Logger) *Neighbor {
	log = log.With(
		"id", p.ID(),
		"localAddr", ps.Conn().LocalMultiaddr(),
		"remoteAddr", ps.Conn().RemoteMultiaddr(),
	)
	return &Neighbor{
		Peer:  p,
		Group: group,

		log: log,

		disconnected:   events.NewEvent(disconnected),
		packetReceived: events.NewEvent(packetReceived),

		ps: ps,
	}
}

// PacketsRead returns number of packets this neighbor has received.
func (n *Neighbor) PacketsRead() uint64 {
	return n.ps.packetsRead.Load()
}

// PacketsWritten returns number of packets this neighbor has sent.
func (n *Neighbor) PacketsWritten() uint64 {
	return n.ps.packetsWritten.Load()
}

func disconnected(handler interface{}, _ ...interface{}) {
	handler.(func())()
}

func packetReceived(handler interface{}, params ...interface{}) {
	handler.(func(*pb.Packet))(params[0].(*pb.Packet))
}

// ConnectionEstablished returns the connection established.
func (n *Neighbor) ConnectionEstablished() time.Time {
	return n.ps.Stat().Opened
}

func (n *Neighbor) readLoop() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {
			// This loop gets terminated when we encounter an error on .read() function call.
			// The error might be caused by another goroutine closing the connection by calling .disconnect() function.
			// Or by a problem with the connection itself.
			// In any case we call .disconnect() after encountering the error,
			// the disconnect call is protected with sync.Once, so in case another goroutine called it before us,
			// we won't execute it twice.
			packet := &pb.Packet{}
			err := n.ps.readPacket(packet)
			if err != nil {
				if isPermanentError(err) {
					if disconnectErr := n.disconnect(); disconnectErr != nil {
						n.log.Warnw("Failed to disconnect", "err", disconnectErr)
					}
					return
				}
				if !isTimeoutError(err) {
					n.log.Debugw("Read error", "err", err)
				}
				continue
			}
			n.packetReceived.Trigger(packet)
		}
	}()
}

func (n *Neighbor) close() {
	if err := n.disconnect(); err != nil {
		n.log.Errorw("Failed to disconnect the neighbor", "err", err)
	}
	n.wg.Wait()
}

func (n *Neighbor) disconnect() (err error) {
	n.disconnectOnce.Do(func() {
		if streamErr := n.ps.Close(); streamErr != nil {
			err = errors.WithStack(streamErr)
		}
		n.log.Info("Connection closed")
		n.disconnected.Trigger()
	})
	return err
}

func isPermanentError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") ||
		errors.Is(err, io.ErrClosedPipe) || errors.Is(err, mux.ErrReset) || errors.Is(err, yamux.ErrStreamClosed) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}
