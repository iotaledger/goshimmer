package dashboard

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/event"
)

type SlotInfo struct {
	Index slot.Index `json:"index"`
	ID    string     `json:"id"`
}

func runSlotsLiveFeed(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[SlotsLiveFeed]", func(ctx context.Context) {
		hook := deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(onSlotCommitted, event.WithWorkerPool(plugin.WorkerPool))

		<-ctx.Done()

		log.Info("Stopping Dashboard[SlotsLiveFeed] ...")
		hook.Unhook()
		log.Info("Stopping Dashboard[SlotsLiveFeed] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func onSlotCommitted(e *notarization.SlotCommittedDetails) {
	broadcastWsBlock(&wsblk{MsgTypeSlotInfo, &SlotInfo{Index: e.Commitment.Index(), ID: e.Commitment.ID().Base58()}})
}
