package dashboard

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/runtime/event"
)

type EpochInfo struct {
	Index epoch.Index `json:"index"`
	ID    string      `json:"id"`
}

func runEpochsLiveFeed(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[EpochsLiveFeed]", func(ctx context.Context) {
		hook := deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Hook(onEpochCommitted, event.WithWorkerPool(plugin.WorkerPool))

		<-ctx.Done()

		log.Info("Stopping Dashboard[EpochsLiveFeed] ...")
		hook.Unhook()
		log.Info("Stopping Dashboard[EpochsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onEpochCommitted(e *notarization.EpochCommittedDetails) {
	broadcastWsBlock(&wsblk{MsgTypeEpochInfo, &EpochInfo{Index: e.Commitment.Index(), ID: e.Commitment.ID().Base58()}})
}
