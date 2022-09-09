package node

import (
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/node/solidification"
	"github.com/iotaledger/goshimmer/packages/node/solidification/parser"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
)

type Node struct {
	network        *network.Network
	parser         *parser.Parser
	protocol       *protocol.Protocol
	solidification *solidification.Solidification

	optsSolidificationOptions []options.Option[solidification.Solidification]

	*logger.Logger
}

func New(networkInstance *network.Network, log *logger.Logger, opts ...options.Option[Node]) (node *Node) {
	return options.Apply(&Node{
		Logger: log,
	}, opts, func(n *Node) {
		n.network = networkInstance
		n.protocol = protocol.New(log)
		n.parser = &parser.Parser{}
		n.solidification = solidification.New(n.protocol, n.network, n.optsSolidificationOptions...)

		n.network.Events.BlockReceived.Attach(network.BlockReceivedHandler(n.protocol.ProcessBlockFromPeer))
	})
}

func (n *Node) RequestBlock(block *blockdag.Block) {
	n.network.RequestBlock(block.ID())
}

func WithSolidificationOptions(opts ...options.Option[solidification.Solidification]) options.Option[Node] {
	return func(n *Node) {
		n.optsSolidificationOptions = opts
	}
}
