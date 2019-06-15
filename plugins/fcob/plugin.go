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

var PLUGIN = node.NewPlugin("FCOB", configure, run)

var runProtocol RunProtocol
var fcob tangleHook

func configure(plugin *node.Plugin) {
	fcob = tangleHook{}
	runProtocol = makeRunProtocol(fpcP.INSTANCE, fcob)
}

func run(plugin *node.Plugin) {
	// subscribe to a new Tx received event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(
		events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
			runProtocol(transaction.GetHash())
		}),
	)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpcP.Events.NewFinalizedTxs.Attach(
		events.NewClosure(func(txs []fpc.TxOpinion) {
			plugin.LogInfo(fmt.Sprintf("Finalized Txs: %v", txs))
			for _, tx := range txs {
				fcob.SetOpinion(ternary.Trinary(tx.TxHash), Opinion{tx.Opinion, true})
				//remove finalized txs from beingVoted map
				beingVoted.Delete(ternary.Trinary(tx.TxHash))
			}
		}))
}
