package network

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Network struct {
	Events *Events

	P2PManager    *p2p.Manager
	WarpSyncMgr   *warpsync.Manager
	gossipManager *gossip.Manager
}

func New(p2pManager *p2p.Manager, blockProvider func(models.BlockID) (*models.Block, bool), logger *logger.Logger, opts ...options.Option[Network]) (network *Network) {
	network = options.Apply(&Network{
		Events:     NewEvents(),
		P2PManager: p2pManager,
	}, opts)

	// TODO: fix types
	// network.WarpSyncMgr = warpsync.NewManager(network.P2PManager, blockProvider, func(block *models.Block, peer *peer.Peer) {
	// 	network.Events.BlockReceived.Trigger(&BlockReceivedEvent{
	// 		Block: block,
	// 		Peer:  peer,
	// 	})
	// }, logger)

	// TODO: fix types
	// network.GossipMgr = gossip.NewManager(network.P2PManager, func(blockId models.BlockID) ([]byte, error) {
	// 	return blockProvider(blockId).Bytes()
	// }, logger)

	network.gossipManager.Events.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
		eventToTrigger := &BlockReceivedEvent{
			Block: new(models.Block),
			Peer:  event.Peer,
		}

		switch _, err := eventToTrigger.Block.FromBytes(event.Data); {
		case err != nil:
			network.Events.InvalidBlockReceived.Trigger(event.Peer)

		default:
			network.Events.BlockReceived.Trigger(eventToTrigger)
		}
	}))

	return network
}

func (n *Network) RequestBlock(id models.BlockID) {
	n.gossipManager.RequestBlock(lo.PanicOnErr(id.Bytes()))
}
