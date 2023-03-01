package p2p

import (
	"context"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
)

// NeighborsGroup is an enum type for various neighbors groups like auto/manual.
type NeighborsGroup int8

const (
	NeighborsSendQueueSize = 1000
)

const (
	// NeighborsGroupAuto represents a neighbors group that is managed automatically.
	NeighborsGroupAuto NeighborsGroup = iota
	// NeighborsGroupManual represents a neighbors group that is managed manually.
	NeighborsGroupManual
)

type queuedPacket struct {
	protocolID protocol.ID
	packet     proto.Message
}

type (
	PacketReceivedFunc       func(neighbor *Neighbor, protocol protocol.ID, packet proto.Message)
	NeighborDisconnectedFunc func(neighbor *Neighbor)
)

// Neighbor describes the established p2p connection to another peer.
type Neighbor struct {
	*peer.Peer
	Group NeighborsGroup

	packetReceivedFunc PacketReceivedFunc
	disconnectedFunc   NeighborDisconnectedFunc

	Log            *logger.Logger
	disconnectOnce sync.Once
	wg             sync.WaitGroup

	loopCtx       context.Context
	loopCtxCancel context.CancelFunc

	// protocols is a map of protocol IDs to their respective PacketsStream.
	// As it is only initialized from the Neighbor constructor, no locking is needed.
	protocols map[protocol.ID]*PacketsStream

	sendQueue chan *queuedPacket
}

// NewNeighbor creates a new neighbor from the provided peer and connection.
func NewNeighbor(p *peer.Peer, group NeighborsGroup, protocols map[protocol.ID]*PacketsStream, log *logger.Logger, packetReceivedCallback PacketReceivedFunc, disconnectedCallback NeighborDisconnectedFunc) *Neighbor {
	ctx, cancel := context.WithCancel(context.Background())

	neighbor := &Neighbor{
		Peer:  p,
		Group: group,

		packetReceivedFunc: packetReceivedCallback,
		disconnectedFunc:   disconnectedCallback,

		loopCtx:       ctx,
		loopCtxCancel: cancel,

		protocols: protocols,
		sendQueue: make(chan *queuedPacket, NeighborsSendQueueSize),
	}

	conn := neighbor.getAnyStream().Conn()

	neighbor.Log = log.With(
		"id", p.ID(),
		"localAddr", conn.LocalMultiaddr(),
		"remoteAddr", conn.RemoteMultiaddr(),
	)

	return neighbor
}

func (n *Neighbor) Enqueue(packet proto.Message, protocolID protocol.ID) {
	select {
	case n.sendQueue <- &queuedPacket{protocolID: protocolID, packet: packet}:
	default:
		n.Log.Warn("Dropped packet due to SendQueue being full")
	}
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
				if n.loopCtx.Err() != nil {
					n.Log.Infof("Exit %s readLoop due to canceled context", protocolID)
					return
				}

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
				n.packetReceivedFunc(n, protocolID, packet)
			}
		}(protocolID, stream)
	}
}

func (n *Neighbor) writeLoop() {
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		for {
			select {
			case <-n.loopCtx.Done():
				n.Log.Info("Exit writeLoop due to canceled context")
				return
			case sendPacket := <-n.sendQueue:
				stream := n.GetStream(sendPacket.protocolID)
				if stream == nil {
					n.Log.Warnw("send error, no stream for protocol", "peer-id", n.ID(), "protocol", sendPacket.protocolID)
					if disconnectErr := n.disconnect(); disconnectErr != nil {
						n.Log.Warnw("Failed to disconnect", "err", disconnectErr)
					}
					return
				}
				if err := stream.WritePacket(sendPacket.packet); err != nil {
					n.Log.Warnw("send error", "peer-id", n.ID(), "err", err)
					if disconnectErr := n.disconnect(); disconnectErr != nil {
						n.Log.Warnw("Failed to disconnect", "err", disconnectErr)
					}
					return
				}
			}
		}
	}()
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
		// Stop the loops
		n.loopCtxCancel()

		// Close all streams
		for _, stream := range n.protocols {
			if streamErr := stream.Close(); streamErr != nil {
				err = errors.WithStack(streamErr)
			}
			n.Log.Infow("Stream closed", "protocol", stream.Protocol())
		}
		n.Log.Info("Connection closed")
		n.disconnectedFunc(n)
	})
	return err
}
