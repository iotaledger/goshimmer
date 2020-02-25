package spa

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transactionmetadata"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		task.Param(0).(*transaction.CachedTransaction).Consume(func(transaction *transaction.Transaction) {
			sendToAllWSClient(&msg{MsgTypeTx, &tx{transaction.GetId().String(), 0}})
		})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	newTxRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewTx := events.NewClosure(func(tx *transaction.CachedTransaction, metadata *transactionmetadata.CachedTransactionMetadata) {
		metadata.Release()

		select {
		case <-newTxRateLimiter.C:
			liveFeedWorkerPool.TrySubmit(tx)
		default:
			tx.Release()
		}
	})

	daemon.BackgroundWorker("SPA[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		tangle.Instance.Events.TransactionAttached.Attach(notifyNewTx)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[TxUpdater] ...")
		tangle.Instance.Events.TransactionAttached.Detach(notifyNewTx)
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.Stop()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
