package tangle

import (
    "fmt"
    "github.com/iotaledger/goshimmer/packages/database"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/packages/transaction"
    "github.com/iotaledger/goshimmer/plugins/gossip"
)

// region plugin module setup //////////////////////////////////////////////////////////////////////////////////////////

var PLUGIN = node.NewPlugin("Tangle", configure, run)

func configure(node *node.Plugin) {
    if db, err := database.Get("transactions"); err != nil {
        panic(err)
    } else {
        transactionDatabase = db
    }

    gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(transaction *transaction.Transaction) {
        if transactionStoredAlready, err := transactionDatabase.Contains(transaction.Hash.ToBytes()); err != nil {
            panic(err)
        } else {
            if !transactionStoredAlready {

                fmt.Println(transaction.Hash.ToString())
                // process transaction
            }
        }
    }))
}

func run(node *node.Plugin) {
    /*
    if accountability.OWN_ID.StringIdentifier == "72f84aaee02af4542672cb25aceb9d9e458ef2a3" {
        go func() {
            txCounter := 0

            for {
                txCounter++

                dummyTx := make([]byte, transaction.MARSHALLED_TOTAL_SIZE / ternary.NUMBER_OF_TRITS_IN_A_BYTE)
                for i := 0; i < 1620; i++ {
                    dummyTx[i] = byte((i + txCounter) % 128)
                }

                gossip.SendTransaction(transaction.FromBytes(dummyTx))

                <- time.After(1000 * time.Millisecond)
            }
        }()
    }*/
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var transactionDatabase database.Database

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
