package network

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/network/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

// region Events ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Events struct {
	BlockReceived        *event.Linkable[*BlockReceivedEvent, Events, *Events]
	InvalidBlockReceived *event.Linkable[*peer.Peer, Events, *Events]
	PeerDropped          *event.Linkable[*peer.Peer, Events, *Events]

	Gossip   *gossip.Events
	WarpSync *warpsync.Events

	event.LinkableCollection[Events, *Events]
}

// NewEvents contains the constructor of the Events object (it is generated by a generic factory).
var NewEvents = event.LinkableConstructor(func() (newEvents *Events) {
	return &Events{
		BlockReceived:        event.NewLinkable[*BlockReceivedEvent, Events, *Events](),
		InvalidBlockReceived: event.NewLinkable[*peer.Peer, Events, *Events](),
		PeerDropped:          event.NewLinkable[*peer.Peer, Events, *Events](),
	}
})

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region BlockReceivedEvent ///////////////////////////////////////////////////////////////////////////////////////////

type BlockReceivedEvent struct {
	Block    *models.Block
	Neighbor *p2p.Neighbor
}

func BlockReceivedHandler(handler func(block *models.Block, neighbor *p2p.Neighbor)) *event.Closure[*BlockReceivedEvent] {
	return event.NewClosure(func(event *BlockReceivedEvent) {
		handler(event.Block, event.Neighbor)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
