package gossip

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
)

var PLUGIN = node.NewPlugin("Gossip", configure, run)

func configure(plugin *node.Plugin) {
    configureNeighbors(plugin)
    configureServer(plugin)

    Events.ReceiveTransaction.Attach(events.NewClosure(func(transaction *transaction.Transaction) {

    }))
}

func run(plugin *node.Plugin) {
    runNeighbors(plugin)
    runServer(plugin)
}
