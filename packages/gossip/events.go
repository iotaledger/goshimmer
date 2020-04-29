package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events defines all the events related to the gossip protocol.
type Events struct {
	// Fired when an attempt to build a connection to a neighbor has failed.
	ConnectionFailed *events.Event
	// Fired when a neighbor connection has been established.
	NeighborAdded *events.Event
	// Fired when a neighbor has been removed.
	NeighborRemoved *events.Event
	// Fired when a new message was received via the gossip protocol.
	MessageReceived *events.Event
}

// MessageReceivedEvent holds data about a message received event.
type MessageReceivedEvent struct {
	// The raw message.
	Data []byte
	// The sender of the message.
	Peer *peer.Peer
}

func peerAndErrorCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer, error))(params[0].(*peer.Peer), params[1].(error))
}

func peerCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer))(params[0].(*peer.Peer))
}

func neighborCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Neighbor))(params[0].(*Neighbor))
}

func messageReceived(handler interface{}, params ...interface{}) {
	handler.(func(*MessageReceivedEvent))(params[0].(*MessageReceivedEvent))
}
