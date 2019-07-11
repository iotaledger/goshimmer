package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("Bundle Processor", configure, run)

func configure(plugin *node.Plugin) {
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		if tx.IsHead() {
			workerPool.Submit(tx)
		}
	}))

	Events.Error.Attach(events.NewClosure(func(err errors.IdentifiableError) {
		plugin.LogFailure(err.Error())
	}))

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogInfo("Stopping Bundle Processor ...")

		workerPool.Stop()

		plugin.LogInfo("Stopping Value Bundle Processor ...")

		valueBundleProcessorWorkerPool.Stop()
	}))
}

func run(plugin *node.Plugin) {
	plugin.LogInfo("Starting Bundle Processor ...")

	daemon.BackgroundWorker("Bundle Processor", func() {
		plugin.LogSuccess("Starting Bundle Processor ... done")

		workerPool.Run()

		plugin.LogSuccess("Stopping Bundle Processor ... done")
	})

	plugin.LogInfo("Starting Value Bundle Processor ...")

	daemon.BackgroundWorker("Value Bundle Processor", func() {
		plugin.LogSuccess("Starting Value Bundle Processor ... done")

		valueBundleProcessorWorkerPool.Run()

		plugin.LogSuccess("Stopping Value Bundle Processor ... done")
	})
}
