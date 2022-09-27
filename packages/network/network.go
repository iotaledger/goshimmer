package network

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/interfaces"
	"github.com/iotaledger/goshimmer/packages/network/chain"
	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/network/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type Network struct {
	P2PManager interfaces.Network

	Chain *chain.Protocol

	events           *Events
	warpSyncProtocol *warpsync.Protocol
	gossipProtocol   *gossip.Manager
}

func New(p2pManager interfaces.Network, logger *logger.Logger, opts ...options.Option[Network]) (network *Network) {
	network = options.Apply(&Network{
		events:         NewEvents(),
		P2PManager:     p2pManager,
		gossipProtocol: gossip.NewManager(p2pManager, logger),
	}, opts)

	network.events.Gossip = network.gossipProtocol.Events

	// TODO: fix types
	// network.WarpSyncMgr = warpsync.NewManager(network.P2PManager, blockProvider, func(block *models.Block, peer *peer.Peer) {
	// 	network.Events.BlockReceived.Trigger(&BlockReceivedEvent{
	// 		Block: block,
	// 		Peer:  peer,
	// 	})
	// }, logger)

	network.P2PManager.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
		network.events.PeerDropped.Trigger(event.Neighbor.Peer)
	}))
	network.P2PManager.NeighborGroupEvents(p2p.NeighborsGroupManual).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
		network.events.PeerDropped.Trigger(event.Neighbor.Peer)
	}))

	return network
}

func (n *Network) Events() *Events {
	return n.events
}

func (n *Network) SendBlock(block *models.Block, peers ...*peer.Peer) {
	n.gossipProtocol.SendBlock(lo.PanicOnErr(block.Bytes()), lo.Map(peers, (*peer.Peer).ID)...)
}

func (n *Network) RequestBlock(id models.BlockID, peers ...*peer.Peer) {
	n.gossipProtocol.RequestBlock(lo.PanicOnErr(id.Bytes()), lo.Map(peers, (*peer.Peer).ID)...)
}

func (n *Network) RequestEpochRange(start, end epoch.Index, startEC commitment.ID, endPrevEC commitment.ID) (err error) {
	// TODO WarpRange ... context.Background()
	return nil
}

func (n *Network) DropPeer(peer *peer.Peer) {
	_ = n.P2PManager.DropNeighbor(peer.ID(), p2p.NeighborsGroupAuto)
	_ = n.P2PManager.DropNeighbor(peer.ID(), p2p.NeighborsGroupManual)
}
