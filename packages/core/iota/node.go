package iota

import (
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/database"
)

type Node struct {
	Database *database.Manager
	Protocol *Protocol
	// Parser
	// Requester
	// Gossip
	// P2P
	// Auto peering
	// Warp sync

	optsDatabaseOptions []database.Option
	optsProtocolOptions []options.Option[Protocol]
}

func NewNode(dbFolder string, opts ...options.Option[Node]) (newNode *Node) {
	return options.Apply(&Node{}, opts, func(n *Node) {
		n.Database = database.NewManager(dbFolder, n.optsDatabaseOptions...)
		n.Protocol = NewProtocol(n.optsProtocolOptions...)
	})
}
