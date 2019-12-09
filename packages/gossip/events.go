package gossip

import (
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events contains all the events related to the gossip protocol.
var Events = struct {
	// A TransactionReceived event is triggered when a new transaction is received by the gossip protocol.
	TransactionReceived *events.Event
	// A NeighborDropped event is triggered when a neighbor has been dropped.
	NeighborDropped *events.Event
	// A RequestTransaction should be triggered for a transaction to be requested through the gossip protocol.
	RequestTransaction *events.Event
}{
	TransactionReceived: events.NewEvent(transactionReceived),
	NeighborDropped:     events.NewEvent(neighborDropped),
	RequestTransaction:  events.NewEvent(requestTransaction),
}

type TransactionReceivedEvent struct {
	Body []byte
	Peer *peer.Peer
}

type RequestTransactionEvent struct {
	Hash []byte // hash of the transaction to request
}
type NeighborDroppedEvent struct {
	Peer *peer.Peer
}

func transactionReceived(handler interface{}, params ...interface{}) {
	handler.(func(*TransactionReceivedEvent))(params[0].(*TransactionReceivedEvent))
}

func requestTransaction(handler interface{}, params ...interface{}) {
	handler.(func(*RequestTransactionEvent))(params[0].(*RequestTransactionEvent))
}

func neighborDropped(handler interface{}, params ...interface{}) {
	handler.(func(*NeighborDroppedEvent))(params[0].(*NeighborDroppedEvent))
}
