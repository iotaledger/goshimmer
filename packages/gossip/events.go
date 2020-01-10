package gossip

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events contains all the events related to the gossip protocol.
var Events = struct {
	// A ConnectionFailed event is triggered when a neighbor connection to a peer could not be established.
	ConnectionFailed *events.Event
	// A NeighborAdded event is triggered when a connection to a new neighbor has been established.
	NeighborAdded *events.Event
	// A NeighborRemoved event is triggered when a neighbor has been dropped.
	NeighborRemoved *events.Event
	// A TransactionReceived event is triggered when a new transaction is received by the gossip protocol.
	TransactionReceived *events.Event
}{
	ConnectionFailed:    events.NewEvent(peerCaller),
	NeighborAdded:       events.NewEvent(neighborCaller),
	NeighborRemoved:     events.NewEvent(peerCaller),
	TransactionReceived: events.NewEvent(transactionReceived),
}

type TransactionReceivedEvent struct {
	Data []byte     // transaction data
	Peer *peer.Peer // peer that send the transaction
}

func peerCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer))(params[0].(*peer.Peer))
}

func neighborCaller(handler interface{}, params ...interface{}) {
	handler.(func(*Neighbor))(params[0].(*Neighbor))
}

func transactionReceived(handler interface{}, params ...interface{}) {
	handler.(func(*TransactionReceivedEvent))(params[0].(*TransactionReceivedEvent))
}
