package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

var (
	liveFeedWorkerCount     = 1
	liveFeedWorkerQueueSize = 50
	liveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		message := task.Param(0).(*tangle.Message)

		broadcastWsMessage(&wsmsg{MsgTypeMessage, &msg{message.ID().Base58(), 0, uint32(message.Payload().Type())}})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			liveFeedWorkerPool.TrySubmit(message)
		})
	})

	if err := daemon.BackgroundWorker("Dashboard[MsgUpdater]", func(ctx context.Context) {
		deps.Tangle.Storage.Events.MessageStored.Attach(notifyNewMsg)
		<-ctx.Done()
		log.Info("Stopping Dashboard[MsgUpdater] ...")
		deps.Tangle.Storage.Events.MessageStored.Detach(notifyNewMsg)
		liveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[MsgUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
