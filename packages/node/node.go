package node

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node/solidification"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
)

type Node struct {
	network        *network.Network
	protocol       *protocol.Protocol
	solidification *solidification.Solidification

	optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(p2pManager *p2p.Manager, log *logger.Logger, opts ...options.Option[Node]) (node *Node) {
	return options.Apply(&Node{
		Logger: log,
	}, opts, func(n *Node) {
		n.network = network.New(p2pManager, n.protocol.Block, log)
		n.protocol = protocol.New(log)
		n.solidification = solidification.New(n.protocol, n.optsSolidificationOptions...)

		n.network.Events.BlockReceived.Attach(network.BlockReceivedHandler(n.protocol.ProcessBlockFromPeer))
	})
}

func (n *Node) RequestBlock(block *blockdag.Block) {
	n.network.RequestBlock(block.ID())
}
