package metrics

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("Metrics", node.Enabled, configure, run)

func configure(plugin *node.Plugin) {
	// increase received TPS counter whenever we receive a new transaction
	tangle.Instance.Events.TransactionAttached.Attach(events.NewClosure(func(transaction *message.CachedTransaction, metadata *transactionmetadata.CachedTransactionMetadata) {
		transaction.Release()
		metadata.Release()

		increaseReceivedTPSCounter()
	}))
}

func run(plugin *node.Plugin) {
	// create a background worker that "measures" the TPS value every second
	daemon.BackgroundWorker("Metrics TPS Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureReceivedTPS, 1*time.Second, shutdownSignal)
	}, shutdown.ShutdownPriorityMetrics)
}
