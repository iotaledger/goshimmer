package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/fpc"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/ternary"
	"github.com/iotaledger/goshimmer/packages/transaction"
	fpcP "github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("FCOB", configure, run)

func configure(plugin *node.Plugin) {

}

func run(plugin *node.Plugin) {
	// subscribe to a new solidified Tx received event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(
		events.NewClosure(func(transaction *transaction.Transaction) {
			runProtocol(transaction.Hash)
		}),
	)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpcP.Events.NewFinalizedTxs.Attach(
		events.NewClosure(func(txs []fpc.TxOpinion) {
			plugin.LogInfo(fmt.Sprintf("Finalized Txs: %v", txs))
			for _, tx := range txs {
				updateOpinion(ternary.Trinary(tx.TxHash).ToTrits(), tx.Opinion, true)
			}
		}))
}
