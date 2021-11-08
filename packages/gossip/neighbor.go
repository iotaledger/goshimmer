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
	"github.com/libp2p/go-libp2p-core/network"
	"go.uber.org/atomic"

	pb "github.com/iotaledger/goshimmer/packages/gossip/gossipproto"
	"github.com/iotaledger/goshimmer/packages/libp2putil"
)

const (
	ioTimeout = 4 * time.Second
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

	stream         network.Stream
	reader         *libp2putil.UvarintReader
	writer         *libp2putil.UvarintWriter
	packetsRead    *atomic.Uint64
	packetsWritten *atomic.Uint64
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, stream network.Stream, log *logger.Logger) *Neighbor {
	log = log.With(
		"id", p.ID(),
		"localAddr", stream.Conn().LocalMultiaddr(),
		"remoteAddr", stream.Conn().RemoteMultiaddr(),
	)
	return &Neighbor{
		Peer:  p,
		Group: group,

		log: log,

		stream: stream,
		reader: libp2putil.NewDelimitedReader(stream),
		writer: libp2putil.NewDelimitedWriter(stream),

		disconnected:   events.NewEvent(disconnected),
		packetReceived: events.NewEvent(packetReceived),
		packetsRead:    atomic.NewUint64(0),
		packetsWritten: atomic.NewUint64(0),
	}
}

// PacketsRead returns number of packets this neighbor has received.
func (n *Neighbor) PacketsRead() uint64 {
	return n.packetsRead.Load()
}

// PacketsWritten returns number of packets this neighbor has sent.
func (n *Neighbor) PacketsWritten() uint64 {
	return n.packetsWritten.Load()
}

func disconnected(handler interface{}, _ ...interface{}) {
	handler.(func())()
}

func packetReceived(handler interface{}, params ...interface{}) {
	handler.(func(*pb.Packet))(params[0].(*pb.Packet))
}

// ConnectionEstablished returns the connection established.
func (n *Neighbor) ConnectionEstablished() time.Time {
	return n.stream.Stat().Opened
}

func (n *Neighbor) readLoop() {
	n.wg.Add(1)
	defer n.wg.Done()
	go func() {
		for {
			packet := &pb.Packet{}
			err := n.read(packet)
			// the problem is here
			if err != nil {
				if !isCloseError(err) && !errors.Is(err, io.EOF) {
					n.log.Warnw("Permanent error", "err", err)
				}
				return
			}
			n.packetReceived.Trigger(packet)
		}
	}()
}

func (n *Neighbor) write(packet *pb.Packet) error {
	if err := n.stream.SetWriteDeadline(time.Now().Add(ioTimeout)); err != nil {
		n.log.Warnw("Can't set write deadline on stream", "err", err)
	}
	err := n.writer.WriteMsg(packet)
	if err != nil {
		disconnectErr := n.disconnect()
		return errors.CombineErrors(err, disconnectErr)
	}
	n.packetsWritten.Inc()
	return nil
}

func (n *Neighbor) read(packet *pb.Packet) error {
	err := n.reader.ReadMsg(packet)
	if err != nil {
		disconnectErr := n.disconnect()
		if isCloseError(err) || errors.Is(err, io.EOF) {
			return nil
		}
		return errors.CombineErrors(err, disconnectErr)
	}
	n.packetsRead.Inc()
	return nil
}

func (n *Neighbor) close() {
	if err := n.disconnect(); err != nil {
		n.log.Errorw("Failed to disconnect the neighbor", "err", err)
	}
	n.wg.Wait()
}

func (n *Neighbor) disconnect() (err error) {
	n.disconnectOnce.Do(func() {
		if streamErr := n.stream.Close(); streamErr != nil {
			err = errors.WithStack(streamErr)
			return
		}
		n.log.Info("Connection closed")
		n.disconnected.Trigger()
	})
	return err
}

func isCloseError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection") || errors.Is(err, io.ErrClosedPipe)
}
