package gossip

import (
	"io"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	libp2pproto "github.com/gogo/protobuf/proto"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio/protoio"

	pb "github.com/iotaledger/goshimmer/packages/gossip/proto"
)

const (
	ioTimeout         = 4 * time.Second
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

	manager        *Manager
	log            *logger.Logger
	disconnectOnce sync.Once
	wg             sync.WaitGroup

	stream network.Stream
	reader protoio.ReadCloser
	writer protoio.WriteCloser
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, stream network.Stream, manager *Manager) *Neighbor {

	log := manager.log.With(
		"id", p.ID(),
		"localAddr", stream.Conn().LocalMultiaddr(),
		"remoteAddr", stream.Conn().RemoteMultiaddr(),
	)
	const maxMsgSize = 1024 * 1024
	return &Neighbor{
		Peer:   p,
		Group:  group,

		manager: manager,
		log:    log,

		stream: stream,
		reader: protoio.NewDelimitedReader(stream, maxMsgSize),
		writer: protoio.NewDelimitedWriter(stream),
	}
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
			if err != nil {
				n.log.Warnw("Permanent error", "err", err)
				return
			}
			if err := n.manager.handlePacket(packet, n); err != nil {
				n.log.Debugw("Can't handle packet", "err", err)
			}
		}
	}()
}


func (n *Neighbor) write(msg libp2pproto.Message) error {
	if err := n.stream.SetWriteDeadline(time.Now().Add(ioTimeout)); err != nil {
		return errors.WithStack(err)
	}
	err := n.writer.WriteMsg(msg)
	if err != nil {
		n.disconnect()
		if isCloseError(err) {
			return nil
		}
		return errors.WithStack(err)
	}
	return nil
}

func (n *Neighbor) read(msg libp2pproto.Message) error {
	if err := n.stream.SetReadDeadline(time.Now().Add(ioTimeout)); err != nil {
		return errors.WithStack(err)
	}
	err := n.reader.ReadMsg(msg)
	if err != nil {
		n.disconnect()
		if isCloseError(err) || errors.Is(err, io.EOF) {
			return nil
		}
		return errors.WithStack(err)
	}
	return nil
}

func (n *Neighbor) close() {
	n.disconnect()
	n.wg.Wait()
}

func (n *Neighbor) disconnect() {
	n.disconnectOnce.Do(func() {
		if err := n.stream.Close(); err != nil {
			n.log.Warnw("Failed to close the stream", "err", err)
			return
		}
		n.log.Info("Connection closed")
		n.manager.deleteNeighbor(n.ID())
		go n.manager.NeighborsEvents(n.Group).NeighborRemoved.Trigger(n)
	})
}

func isCloseError(err error) bool {
	return strings.Contains(err.Error(), "use of closed network connection")
}
