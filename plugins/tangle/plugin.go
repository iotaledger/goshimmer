package tangle

import (
    "github.com/iotaledger/goshimmer/packages/database"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
    "github.com/iotaledger/goshimmer/plugins/gossip"
)

var PLUGIN = node.NewPlugin("Tangle", configure, run)

func configure(node *node.Plugin) {
    gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(transaction *transaction.Transaction) {
        if transactionStoredAlready, err := transactionDatabase.Contains(transaction.Hash.ToBytes()); err != nil {
            panic(err)
        } else {
            if !transactionStoredAlready {
                // process transaction
            }
        }
    }))
}

func run(node *node.Plugin) {

}

var transactionDatabase database.Database

func init() {
    if db, err := database.Get("transactions"); err != nil {
        panic(err)
    } else {
        transactionDatabase = db
    }
}
