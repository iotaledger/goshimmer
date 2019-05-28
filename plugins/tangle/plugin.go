package tangle

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/iotaledger/goshimmer/packages/accountability"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
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

				fmt.Println("--------- tx received ------")
				fmt.Println("Hash ", transaction.Hash.ToString())
				fmt.Println("at   ", accountability.OWN_ID.StringIdentifier)
				// process transaction
			}
		}
	}))
}

func run(node *node.Plugin) {
	// for now just gossip the same tx in all nodes,
	// for the simulated nodes there will be no reaction to the gossip sending (because the master node sent it already)
	// if accountability.OWN_ID.StringIdentifier == "72f84aaee02af4542672cb25aceb9d9e458ef2a3" {
	if true {
		go func() {
			rand.Seed(time.Now().UTC().UnixNano()) // ??? this seed needs to be increased by something that is private to the node
			txCounter := rand.Intn(128) % 128      // since the nodes are started slightly delayed this should provide randomness

			for {
				txCounter++

				// create a "random" byte message
				dummyTx := make([]byte, transaction.MARSHALLED_TOTAL_SIZE/ternary.NUMBER_OF_TRITS_IN_A_BYTE)
				// for i := 0; i < 1620; i++ {  // the value below should be 1620
				for i := 0; i < transaction.MARSHALLED_TOTAL_SIZE/ternary.NUMBER_OF_TRITS_IN_A_BYTE; i++ {
					dummyTx[i] = byte((i + txCounter) % 128)
				}

				// convert byte message and gossip
				gossip.SendTransaction(transaction.FromBytes(dummyTx))
				fmt.Println("--- new tx --- ")
				fmt.Println("Hash ", ternary.TritsToString(transaction.FromBytes(dummyTx).Hash, 0, len(transaction.FromBytes(dummyTx).Hash)))
				fmt.Println("from ", accountability.OWN_ID.StringIdentifier)

				// wait x seconds till the next tx
				<-time.After(20000 * time.Millisecond)
			}
		}()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region constants and variables //////////////////////////////////////////////////////////////////////////////////////

var transactionDatabase database.Database

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
