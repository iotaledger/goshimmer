package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	fpcP "github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("FCOB", configure, run)

var runProtocol RunProtocol
var fcob Updater

func configure(plugin *node.Plugin) {
	fcob = Updater{}
	runProtocol = makeRunProtocol(fpcP.INSTANCE, fcob)
}

func run(plugin *node.Plugin) {
	// subscribe to a new solidified Tx received event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(
		events.NewClosure(func(transaction *tangle.Transaction) {
			runProtocol(transaction.GetHash())
		}),
	)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpcP.Events.NewFinalizedTxs.Attach(
		events.NewClosure(func(txs []fpc.TxOpinion) {
			plugin.LogInfo(fmt.Sprintf("Finalized Txs: %v", txs))
			for _, tx := range txs {
				fcob.UpdateOpinion(ternary.Trinary(tx.TxHash), tx.Opinion, true)
			}
		}))
}
