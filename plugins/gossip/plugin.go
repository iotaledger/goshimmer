package gossip

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
)

var PLUGIN = node.NewPlugin("Gossip", configure, run)

func configure(plugin *node.Plugin) {
	configureNeighbors(plugin)
	configureServer(plugin)
	configureSendQueue(plugin)

	Events.ReceiveTransaction.Attach(events.NewClosure(func(tx *meta_transaction.MetaTransaction) {
		plugin.LogDebug("Received TX " + string(tx.GetHash()))
	}))
}

func run(plugin *node.Plugin) {
	runNeighbors(plugin)
	runServer(plugin)
	runSendQueue(plugin)
}
