package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/node/shutdown"
)

var (
	liveFeedWorkerCount     = 1
	liveFeedWorkerQueueSize = 50
	liveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		block := task.Param(0).(*tangleold.Block)

		broadcastWsBlock(&wsblk{MsgTypeBlock, &blk{block.ID().Base58(), 0, uint32(block.Payload().Type())}})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	notifyNewBlk := event.NewClosure(func(event *tangleold.BlockStoredEvent) {
		liveFeedWorkerPool.TrySubmit(event.Block)
	})

	if err := daemon.BackgroundWorker("Dashboard[BlkUpdater]", func(ctx context.Context) {
		deps.Tangle.Storage.Events.BlockStored.Attach(notifyNewBlk)
		<-ctx.Done()
		log.Info("Stopping Dashboard[BlkUpdater] ...")
		deps.Tangle.Storage.Events.BlockStored.Detach(notifyNewBlk)
		liveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[BlkUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
