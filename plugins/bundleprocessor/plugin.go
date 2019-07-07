package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/workerpool"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var PLUGIN = node.NewPlugin("Bundle Processor", configure, run)

func configure(plugin *node.Plugin) {
	workerPool = workerpool.New(func(task workerpool.Task) {
		if _, err := ProcessSolidBundleHead(task.Param(0).(*value_transaction.ValueTransaction)); err != nil {
			plugin.LogFailure(err.Error())
		}
	}, workerpool.WorkerCount(WORKER_COUNT), workerpool.QueueSize(2*WORKER_COUNT))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		if tx.IsHead() {
			workerPool.Submit(tx)
		}
	}))

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		plugin.LogInfo("Stopping Bundle Processor ...")

		workerPool.Stop()
	}))
}

func run(plugin *node.Plugin) {
	plugin.LogInfo("Starting Bundle Processor ...")

	daemon.BackgroundWorker("Bundle Processor", func() {
		plugin.LogSuccess("Starting Bundle Processor ... done")

		workerPool.Run()

		plugin.LogSuccess("Stopping Bundle Processor ... done")
	})
}

var workerPool *workerpool.WorkerPool

const WORKER_COUNT = 10000
