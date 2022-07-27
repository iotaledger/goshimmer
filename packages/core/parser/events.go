package tangleold

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

// Events represents events happening in the Parser.
type Events struct {
	// Fired when a block was parsed.
	BlockParsed *event.Event[*BlockParsedEvent]

	// Fired when submitted bytes are rejected by a filter.
	BytesRejected *event.Event[*BytesRejectedEvent]

	// Fired when a block got rejected by a filter.
	BlockRejected *event.Event[*BlockRejectedEvent]
}

func newEvents() *Events {
	return &Events{
		BlockParsed:   event.New[*BlockParsedEvent](),
		BytesRejected: event.New[*BytesRejectedEvent](),
		BlockRejected: event.New[*BlockRejectedEvent](),
	}
}

// BytesRejectedEvent holds the information provided by the BytesRejected event that gets triggered when the bytes of a
// Block did not pass the parsing step.
type BytesRejectedEvent struct {
	Bytes []byte
	Peer  *peer.Peer
	Error error
}

// BlockRejectedEvent holds the information provided by the BlockRejected event that gets triggered when the Block
// was detected to be invalid.
type BlockRejectedEvent struct {
	Block *tangle.Block
	Peer  *peer.Peer
	Error error
}

// BlockParsedEvent holds the information provided by the BlockParsed event that gets triggered when a block was
// fully parsed and syntactically validated.
type BlockParsedEvent struct {
	// Block contains the parsed Block.
	Block *tangle.Block

	// Peer contains the node that sent this Block to the node.
	Peer *peer.Peer
}
