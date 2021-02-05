package dashboard

import (
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var liveFeedWorkerCount = 1
var liveFeedWorkerQueueSize = 50
var liveFeedWorkerPool *workerpool.WorkerPool

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.New(func(task workerpool.Task) {
		message := task.Param(0).(*tangle.Message)

		broadcastWsMessage(&wsmsg{MsgTypeMessage, &msg{message.ID().String(), 0, uint32(message.Payload().Type())}})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	notifyNewMsg := events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			liveFeedWorkerPool.TrySubmit(message)
		})
	})

	if err := daemon.BackgroundWorker("Dashboard[MsgUpdater]", func(shutdownSignal <-chan struct{}) {
		messagelayer.Tangle().Storage.Events.MessageStored.Attach(notifyNewMsg)
		liveFeedWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[MsgUpdater] ...")
		messagelayer.Tangle().Storage.Events.MessageStored.Detach(notifyNewMsg)
		liveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[MsgUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
