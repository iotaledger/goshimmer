package spa

import (
	"time"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		task.Param(0).(*message.CachedMessage).Consume(func(transaction *message.Message) {
			sendToAllWSClient(&msg{MsgTypeTx, &tx{transaction.GetId().String(), 0}})
		})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	newTxRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewTx := events.NewClosure(func(tx *message.CachedMessage, metadata *tangle.CachedMessageMetadata) {
		metadata.Release()

		select {
		case <-newTxRateLimiter.C:
			liveFeedWorkerPool.TrySubmit(tx)
		default:
			tx.Release()
		}
	})

	daemon.BackgroundWorker("SPA[TxUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle.Events.TransactionAttached.Attach(notifyNewTx)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[TxUpdater] ...")
		messagelayer.Tangle.Events.TransactionAttached.Detach(notifyNewTx)
		newTxRateLimiter.Stop()
		liveFeedWorkerPool.Stop()
		log.Info("Stopping SPA[TxUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
