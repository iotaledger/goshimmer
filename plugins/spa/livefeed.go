package spa

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	tangle_plugin "github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		t := task.Param(0).(*value_transaction.ValueTransaction)
		sendToAllWSClient(&msg{MsgTypeTx, &tx{t.GetHash(), t.GetValue()}})
		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	newTxRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewTx := events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		select {
		case <-newTxRateLimiter.C:
			liveFeedWorkerPool.TrySubmit(tx)
		default:
		}
	})

	daemon.BackgroundWorker("SPA[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle_plugin.Events.TransactionStored.Attach(notifyNewTx)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[TxUpdater] ...")
		tangle_plugin.Events.TransactionStored.Detach(notifyNewTx)
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.Stop()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
