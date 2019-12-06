package gossip

import (
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events contains all the events that are triggered during the gossip protocol.
var Events = struct {
	// A NewTransaction event is triggered when a new transaction is received by the gossip protocol.
	NewTransaction *events.Event
	DropNeighbor   *events.Event
}{
	NewTransaction: events.NewEvent(newTransaction),
	DropNeighbor:   events.NewEvent(dropNeighbor),
}

type NewTransactionEvent struct {
	Body []byte
	Peer *peer.Peer
}
type DropNeighborEvent struct {
	Peer *peer.Peer
}

func newTransaction(handler interface{}, params ...interface{}) {
	handler.(func(*NewTransactionEvent))(params[0].(*NewTransactionEvent))
}

func dropNeighbor(handler interface{}, params ...interface{}) {
	handler.(func(*DropNeighborEvent))(params[0].(*DropNeighborEvent))
}
