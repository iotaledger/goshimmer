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

var drngLiveFeedWorkerCount = 1
var drngLiveFeedWorkerQueueSize = 50
var drngLiveFeedWorkerPool *workerpool.WorkerPool

func configureDrngLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		task.Param(0).(*message.CachedMessage).Consume(func(message *message.Message) {
			sendToAllWSClient(&msg{MsgTypeTx, &tx{message.Id().String(), 0}})
		})

		task.Return(nil)
	}, workerpool.WorkerCount(drngLiveFeedWorkerCount), workerpool.QueueSize(drngLiveFeedWorkerQueueSize))
}

func runDrngLiveFeed() {
	newMsgRateLimiter := time.NewTicker(time.Second / 10)
	notifyNewMsg := events.NewClosure(func(message *message.CachedMessage, metadata *tangle.CachedMessageMetadata) {
		metadata.Release()

		select {
		case <-newMsgRateLimiter.C:
			drngLiveFeedWorkerPool.TrySubmit(message)
		default:
			message.Release()
		}
	})

	daemon.BackgroundWorker("SPA[DrngUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle.Events.TransactionAttached.Attach(notifyNewMsg)
		drngLiveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping SPA[DrngUpdater] ...")
		messagelayer.Tangle.Events.TransactionAttached.Detach(notifyNewMsg)
		newMsgRateLimiter.Stop()
		drngLiveFeedWorkerPool.Stop()
		log.Info("Stopping SPA[DrngUpdater] ... done")
	}, shutdown.ShutdownPrioritySPA)
}
