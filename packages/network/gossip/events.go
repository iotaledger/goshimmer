package gossip

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
)

// Events defines all the events related to the gossip protocol.
type Events struct {
	// Fired when a new block was received via the gossip protocol.
	BlockReceived *event.Event[*BlockReceivedEvent]
}

// BlockReceivedEvent holds data about a block received event.
type BlockReceivedEvent struct {
	// The raw block.
	Data []byte
	// The sender of the block.
	Peer *peer.Peer
}

// newEvents returns a new Events object.
func newEvents() (new *Events) {
	return &Events{
		BlockReceived: event.New[*BlockReceivedEvent](),
	}
}
