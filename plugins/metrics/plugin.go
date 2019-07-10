package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/meta_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

// create configure handler (get's called when the PLUGIN is "loaded" by the node)
func configure(plugin *node.Plugin) {
	// increase received TPS counter whenever we receive a new transaction
	gossip.Events.ReceiveTransaction.Attach(events.NewClosure(func(_ *meta_transaction.MetaTransaction) { increaseReceivedTPSCounter() }))
}

// create run handler (get's called when the PLUGIN is "executed" by the node)
func run(plugin *node.Plugin) {
	// create a background worker that "measures" the TPS value every second
	daemon.BackgroundWorker("Metrics TPS Updater", func() { timeutil.Ticker(measureReceivedTPS, 1*time.Second) })
}

// export plugin
var PLUGIN = node.NewPlugin("Metrics", configure, run)
