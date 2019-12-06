package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Metrics", node.Enabled, configure, run)

func configure(plugin *node.Plugin) {
	// increase received TPS counter whenever we receive a new transaction
	gossip.Events.NewTransaction.Attach(events.NewClosure(func(_ *gossip.NewTransactionEvent) { increaseReceivedTPSCounter() }))
}

func run(plugin *node.Plugin) {
	// create a background worker that "measures" the TPS value every second
	daemon.BackgroundWorker("Metrics TPS Updater", func() { timeutil.Ticker(measureReceivedTPS, 1*time.Second) })
}
