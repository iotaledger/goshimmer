package dashboard

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/workerpool"
)

var (
	epochsLiveFeedWorkerPool      *workerpool.NonBlockingQueuedWorkerPool
	epochsLiveFeedWorkerCount     = 1
	epochsLiveFeedWorkerQueueSize = 50
)

type EpochInfo struct {
	Index epoch.Index `json:"index"`
	ID    string      `json:"id"`
}

func configureEpochLiveFeed() {
	epochsLiveFeedWorkerPool = workerpool.NewNonBlockingQueuedWorkerPool(func(task workerpool.Task) {
		broadcastWsBlock(&wsblk{task.Param(0).(byte), task.Param(1)})
		task.Return(nil)
	}, workerpool.WorkerCount(epochsLiveFeedWorkerCount), workerpool.QueueSize(conflictsLiveFeedWorkerQueueSize))
}

func runEpochsLiveFeed() {
	if err := daemon.BackgroundWorker("Dashboard[EpochsLiveFeed]", func(ctx context.Context) {
		defer epochsLiveFeedWorkerPool.Stop()

		onEpochCommittedClosure := event.NewClosure(onEpochCommitted)
		deps.Protocol.Engine().NotarizationManager.Events.EpochCommitted.Attach(onEpochCommittedClosure)

		<-ctx.Done()

		log.Info("Stopping Dashboard[EpochsLiveFeed] ...")
		deps.Protocol.Engine().NotarizationManager.Events.EpochCommitted.Detach(onEpochCommittedClosure)
		log.Info("Stopping Dashboard[EpochsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onEpochCommitted(e *notarization.EpochCommittedDetails) {
	epochsLiveFeedWorkerPool.TrySubmit(MsgTypeEpochInfo, &EpochInfo{Index: e.Commitment.Index(), ID: e.Commitment.ID().Base58()})
}
