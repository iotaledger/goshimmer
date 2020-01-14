package bundleprocessor

import (
	"github.com/iotaledger/goshimmer/packages/errors"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Bundle Processor", node.Enabled, configure, run)
var log *logger.Logger

func configure(*node.Plugin) {
	log = logger.NewLogger("Bundle Processor")

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		if tx.IsHead() {
			workerPool.Submit(tx)
		}
	}))

	Events.Error.Attach(events.NewClosure(func(err errors.IdentifiableError) {
		log.Error(err.Error())
	}))
}

func run(*node.Plugin) {
	log.Info("Starting Bundle Processor ...")

	daemon.BackgroundWorker("Bundle Processor", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Bundle Processor ... done")
		workerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Bundle Processor ...")
		workerPool.StopAndWait()
		log.Info("Stopping Bundle Processor ... done")
	}, shutdown.ShutdownPriorityBundleProcessor)

	log.Info("Starting Value Bundle Processor ...")

	daemon.BackgroundWorker("Value Bundle Processor", func(shutdownSignal <-chan struct{}) {
		log.Info("Starting Value Bundle Processor ... done")
		valueBundleProcessorWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Value Bundle Processor ...")
		valueBundleProcessorWorkerPool.StopAndWait()
		log.Info("Stopping Value Bundle Processor ... done")
	}, shutdown.ShutdownPriorityBundleProcessor)
}
