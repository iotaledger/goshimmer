package p2p

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/libp2p/go-libp2p-core/protocol"
)

// NeighborsGroup is an enum type for various neighbors groups like auto/manual.
type NeighborsGroup int8

const (
	// NeighborsGroupAuto represents a neighbors group that is managed automatically.
	NeighborsGroupAuto NeighborsGroup = iota
	// NeighborsGroupManual represents a neighbors group that is managed manually.
	NeighborsGroupManual
)

// Neighbor describes the established p2p connection to another peer.
type Neighbor struct {
	*peer.Peer
	Group NeighborsGroup

	Events *NeighborEvents

	Log            *logger.Logger
	disconnectOnce sync.Once
	wg             sync.WaitGroup

	// protocols is a map of protocol IDs to their respective PacketsStream.
	// As it is only initialized from the Neighbor constructor, no locking is needed.
	protocols map[protocol.ID]*PacketsStream
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, protocols map[protocol.ID]*PacketsStream, log *logger.Logger) *Neighbor {
	new := &Neighbor{
		Peer:  p,
		Group: group,

		Events: NewNeighborEvents(),

		protocols: protocols,
	}

	conn := new.getAnyStream().Conn()

	new.Log = log.With(
		"id", p.ID(),
		"localAddr", conn.LocalMultiaddr(),
		"remoteAddr", conn.RemoteMultiaddr(),
	)

	return new
}

// GetStream returns the stream for the given protocol.
func (n *Neighbor) GetStream(protocol protocol.ID) *PacketsStream {
	return n.protocols[protocol]
}

// PacketsRead returns number of packets this neighbor has received.
func (n *Neighbor) PacketsRead() (count uint64) {
	for _, stream := range n.protocols {
		count += stream.packetsRead.Load()
	}
	return count
}

// PacketsWritten returns number of packets this neighbor has sent.
func (n *Neighbor) PacketsWritten() (count uint64) {
	for _, stream := range n.protocols {
		count += stream.packetsWritten.Load()
	}
	return count
}

// ConnectionEstablished returns the connection established.
func (n *Neighbor) ConnectionEstablished() time.Time {
	return n.getAnyStream().Stat().Opened
}

func (n *Neighbor) getAnyStream() *PacketsStream {
	for _, stream := range n.protocols {
		return stream
	}
	return nil
}

func (n *Neighbor) readLoop() {
	for protocolID, stream := range n.protocols {
		n.wg.Add(1)
		go func(protocolID protocol.ID, stream *PacketsStream) {
			defer n.wg.Done()
			for {
				// This loop gets terminated when we encounter an error on .read() function call.
				// The error might be caused by another goroutine closing the connection by calling .disconnect() function.
				// Or by a problem with the connection itself.
				// In any case we call .disconnect() after encountering the error,
				// the disconnect call is protected with sync.Once, so in case another goroutine called it before us,
				// we won't execute it twice.
				packet := stream.packetFactory()
				err := stream.ReadPacket(packet)
				if err != nil {
					n.Log.Infow("Stream read packet error", "err", err)
					if disconnectErr := n.disconnect(); disconnectErr != nil {
						n.Log.Warnw("Failed to disconnect", "err", disconnectErr)
					}
					return
				}
				n.Events.PacketReceived.Trigger(&NeighborPacketReceivedEvent{
					Neighbor: n,
					Protocol: protocolID,
					Packet:   packet,
				})
			}
		}(protocolID, stream)
	}
}

// Close closes the connection with the neighbor.
func (n *Neighbor) Close() {
	if err := n.disconnect(); err != nil {
		n.Log.Errorw("Failed to disconnect the neighbor", "err", err)
	}
	n.wg.Wait()
}

func (n *Neighbor) disconnect() (err error) {
	n.disconnectOnce.Do(func() {
		for _, stream := range n.protocols {
			if streamErr := stream.Close(); streamErr != nil {
				err = errors.WithStack(streamErr)
			}
			n.Log.Infow("Stream closed", "protocol", stream.Protocol())
		}
		n.Log.Info("Connection closed")
		n.Events.Disconnected.Trigger(&NeighborDisconnectedEvent{})
	})
	return err
}
