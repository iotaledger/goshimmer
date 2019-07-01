package fcob

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

// PLUGIN is the exposed FCoB plugin
var PLUGIN = node.NewPlugin("FCOB", configure, run)

// runProtocol is the FCoB core logic function
var runProtocol *events.Closure
var updateTxsVoted *events.Closure
var api tangleStore

func configure(plugin *node.Plugin) {
	api = tangleStore{}
	runProtocol = makeRunFCOB(plugin, api, fpc.INSTANCE)
	updateTxsVoted = makeUpdateTxsVoted(plugin)
}

func run(plugin *node.Plugin) {
	// subscribe to a new Tx solid event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(runProtocol)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpc.Events.VotingDone.Attach(updateTxsVoted)
}
