package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	fpcP "github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

// PLUGIN is the exposed FCoB plugin
var PLUGIN = node.NewPlugin("FCOB", configure, run)

// runProtocol is the FCoB core logic function
var runProtocol RunProtocol
var db tangleDB

func configure(plugin *node.Plugin) {
	db = tangleDB{}
	runProtocol = makeRunProtocol(plugin, db, fpcP.INSTANCE)
}

func run(plugin *node.Plugin) {
	// subscribe to a new Tx solid event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(
		events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
			// start as a goroutine so that immidiately returns
			go func() {
				err := runProtocol(transaction.GetHash())
				if err != nil {
					plugin.LogFailure(fmt.Sprint(err))
				}
			}()
		}),
	)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpcP.Events.VotingDone.Attach(
		events.NewClosure(func(txs []fpc.TxOpinion) {
			plugin.LogInfo(fmt.Sprintf("Voting Done for txs: %v", txs))
			for _, tx := range txs {
				// update "liked" and "voted" status for all the received txs
				setOpinion(ternary.Trinary(tx.TxHash), Opinion{tx.Opinion, VOTED}, db)
			}
		}))
}
