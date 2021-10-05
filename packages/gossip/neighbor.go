package gossip

import (
	"time"

	"github.com/cockroachdb/errors"
	libp2pproto "github.com/gogo/protobuf/proto"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-msgio/protoio"
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
	Group           NeighborsGroup

	stream          network.Stream
	reader          protoio.ReadCloser
	writer          protoio.WriteCloser
	log             *logger.Logger
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, stream network.Stream, log *logger.Logger) *Neighbor {
	log = log.With(
		"id", p.ID(),
		"localAddr", stream.Conn().LocalMultiaddr(),
		"remoteAddr", stream.Conn().RemoteMultiaddr(),
	)
	const maxMsgSize = 1024 * 1024
	return &Neighbor{
		Peer:   p,
		Group:  group,
		stream: stream,
		log:    log,
		reader: protoio.NewDelimitedReader(stream, maxMsgSize),
		writer: protoio.NewDelimitedWriter(stream),
	}
}

// ConnectionEstablished returns the connection established.
func (n *Neighbor) ConnectionEstablished() time.Time {
	return n.stream.Stat().Opened
}


func (n *Neighbor) Write(msg libp2pproto.Message) error {
	if err := n.stream.SetWriteDeadline(time.Now().Add(ioTimeout)); err != nil {
		return errors.WithStack(err)
	}
	err := n.writer.WriteMsg(msg)
	if err != nil {
		return errors.WithStack(err)
	}
	return nil
}

func (n *Neighbor) Read(msg libp2pproto.Message) error {
	if err := n.stream.SetReadDeadline(time.Now().Add(ioTimeout)); err != nil {
		return errors.WithStack(err)
	}
	err := n.reader.ReadMsg(msg)
	return errors.WithStack(err)
}

func (n *Neighbor) Close() {
	if err := n.stream.Close(); err != nil {
		n.log.Warnw("Failed to close the stream", "err", err)
		return
	}
	 n.log.Info("Connection closed")
}
