package gossip

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/iota.go/trinary"
)

// Events contains all the events related to the gossip protocol.
var Events = struct {
	// A NeighborDropped event is triggered when a neighbor has been dropped.
	NeighborDropped *events.Event
	// A TransactionReceived event is triggered when a new transaction is received by the gossip protocol.
	TransactionReceived *events.Event
}{
	NeighborDropped:     events.NewEvent(neighborDropped),
	TransactionReceived: events.NewEvent(transactionReceived),
}

type NeighborDroppedEvent struct {
	Peer *peer.Peer
}

type TransactionReceivedEvent struct {
	Data []byte     // transaction data
	Peer *peer.Peer // peer that send the transaction
}

type RequestTransactionEvent struct {
	Hash trinary.Trytes // hash of the transaction to request
}

func neighborDropped(handler interface{}, params ...interface{}) {
	handler.(func(*NeighborDroppedEvent))(params[0].(*NeighborDroppedEvent))
}

func transactionReceived(handler interface{}, params ...interface{}) {
	handler.(func(*TransactionReceivedEvent))(params[0].(*TransactionReceivedEvent))
}
