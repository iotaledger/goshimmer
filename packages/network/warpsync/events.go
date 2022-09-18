package warpsync

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// Events defines all the events related to the gossip protocol.
type Events struct {
	// Fired when a new block was received via the gossip protocol.
	EpochCommitmentReceived    *event.Linkable[*EpochCommitmentReceivedEvent, Events, *Events]
	EpochBlocksRequestReceived *event.Linkable[*EpochBlocksRequestReceivedEvent, Events, *Events]
	EpochBlocksStart           *event.Linkable[*EpochBlocksStartEvent, Events, *Events]
	EpochBlock                 *event.Linkable[*EpochBlockEvent, Events, *Events]
	EpochBlocksEnd             *event.Linkable[*EpochBlocksEndEvent, Events, *Events]

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		EpochCommitmentReceived: event.NewLinkable[*EpochCommitmentReceivedEvent, Events, *Events](),
	}
})

// BlockReceivedEvent holds data about a block received event.
type EpochCommitmentReceivedEvent struct {
	Neighbor *p2p.Neighbor
	ECRecord *commitment.Commitment
}

// BlockReceivedEvent holds data about a block received event.
type EpochBlocksRequestReceivedEvent struct {
	Neighbor *p2p.Neighbor
	EI       epoch.Index
	EC       commitment.ID
}

// BlockReceivedEvent holds data about a block received event.
type EpochBlocksStartEvent struct {
	Neighbor *p2p.Neighbor
	EI       epoch.Index
}

// BlockReceivedEvent holds data about a block received event.
type EpochBlockEvent struct {
	Neighbor *p2p.Neighbor
	EI       epoch.Index
	Block    *models.Block
}

// BlockReceivedEvent holds data about a block received event.
type EpochBlocksEndEvent struct {
	Neighbor          *p2p.Neighbor
	EI                epoch.Index
	EC                commitment.ID
	StateMutationRoot commitment.MerkleRoot
	StateRoot         commitment.MerkleRoot
	ManaRoot          commitment.MerkleRoot
}
