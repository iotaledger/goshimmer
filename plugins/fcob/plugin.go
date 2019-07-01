package fcob

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

// PLUGIN is the exposed FCoB plugin
var PLUGIN = node.NewPlugin("FCOB", configure, run)

// runProtocol is the FCoB core logic function
var runProtocol RunProtocol

var api tangleStore

var updateTxsVoted *events.Closure

func configure(plugin *node.Plugin) {
	api = tangleStore{}
	runProtocol = makeRunProtocol(plugin, api, fpc.INSTANCE)
	updateTxsVoted = makeUpdateTxsVoted(plugin)
}

func run(plugin *node.Plugin) {
	// subscribe to a new Tx solid event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(
		events.NewClosure(func(transaction *value_transaction.ValueTransaction) {
			// start as a goroutine so that immediately returns
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
	fpc.Events.VotingDone.Attach(updateTxsVoted)
}
