package main

import (
    "github.com/iotaledger/goshimmer/packages/database"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
    "github.com/iotaledger/goshimmer/plugins/analysis"
    "github.com/iotaledger/goshimmer/plugins/autopeering"
    "github.com/iotaledger/goshimmer/plugins/cli"
    "github.com/iotaledger/goshimmer/plugins/gossip"
    "github.com/iotaledger/goshimmer/plugins/gracefulshutdown"
    "github.com/iotaledger/goshimmer/plugins/statusscreen"
)

func main() {
    node.Run(
        cli.PLUGIN,
        autopeering.PLUGIN,
        gossip.PLUGIN,
        analysis.PLUGIN,
        statusscreen.PLUGIN,
        gracefulshutdown.PLUGIN,
    )

    db, _ := database.Get("transactions")
    gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(tx *transaction.Transaction) {
        db.Set(tx.Hash.ToBytes(), tx.Bytes)
    }))
}
