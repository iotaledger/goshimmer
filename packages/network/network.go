package network

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network/gossip"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node/warpsync"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type Network struct {
	Events *Events

	P2PManager  *p2p.Manager
	WarpSyncMgr *warpsync.Manager
	GossipMgr   *gossip.Manager
}

func New(p2pManager *p2p.Manager, blockProvider func(models.BlockID) (*models.Block, bool), logger *logger.Logger, opts ...options.Option[Network]) (network *Network) {
	network = options.Apply(&Network{
		Events:     NewEvents(),
		P2PManager: p2pManager,
	}, opts)

	network.WarpSyncMgr = warpsync.NewManager(network.P2PManager, blockProvider, func(block *models.Block, peer *peer.Peer) {
		network.Events.BlockReceived.Trigger(&BlockReceivedEvent{
			Block: block,
			Peer:  peer,
		})
	}, logger)

	network.GossipMgr = gossip.NewManager(network.P2PManager, blockProvider, logger)

	return network
}

func (n *Network) Start() {

}
