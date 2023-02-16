package dashboard

import (
	"context"

	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

var (
	liveFeedWorkerCount     = 1
	liveFeedWorkerQueueSize = 50
	liveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
)

func configureLiveFeed() {
	liveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		block := task.Param(0).(*models.Block)

		broadcastWsBlock(&wsblk{MsgTypeBlock, &blk{block.ID().Base58(), 0, block.Payload().Type()}})

		task.Return(nil)
	}, workerpool.WorkerCount(liveFeedWorkerCount), workerpool.QueueSize(liveFeedWorkerQueueSize))
}

func runLiveFeed() {
	notifyNewBlk := event.NewClosure(func(block *blockdag.Block) {
		liveFeedWorkerPool.TrySubmit(block.ModelsBlock)
	})

	if err := daemon.BackgroundWorker("Dashboard[BlkUpdater]", func(ctx context.Context) {
		deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(notifyNewBlk)
		<-ctx.Done()
		log.Info("Stopping Dashboard[BlkUpdater] ...")
		deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Detach(notifyNewBlk)
		liveFeedWorkerPool.Stop()
		log.Info("Stopping Dashboard[BlkUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
