package fcob

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/fpc"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

// PLUGIN is the exposed FCoB plugin
var PLUGIN = node.NewPlugin("FCOB", configure, run)

var api tangleStore
var runFCOB *events.Closure
var updateTxsVoted *events.Closure

func configure(plugin *node.Plugin) {
	api = tangleStore{}
	runFCOB = configureFCOB(plugin, api, fpc.INSTANCE)
	updateTxsVoted = configureUpdateTxsVoted(plugin, api)
}

func run(plugin *node.Plugin) {
	// subscribe to a new Tx solid event
	// and start an instance of the FCoB protocol
	tangle.Events.TransactionSolid.Attach(runFCOB)

	// subscribe to a new VotingDone event
	// and update the related txs opinion
	fpc.Events.VotingDone.Attach(updateTxsVoted)
}
