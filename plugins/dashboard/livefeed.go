package dashboard

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/runtime/event"
)

func runLiveFeed(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker("Dashboard[BlkUpdater]", func(ctx context.Context) {
		hook := deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
			broadcastWsBlock(&wsblk{MsgTypeBlock, &blk{block.ID().Base58(), 0, block.Payload().Type()}})
		}, event.WithWorkerPool(plugin.WorkerPool))
		<-ctx.Done()
		log.Info("Stopping Dashboard[BlkUpdater] ...")
		hook.Unhook()
		log.Info("Stopping Dashboard[BlkUpdater] ... done")
	}, shutdown.PriorityDashboard); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
