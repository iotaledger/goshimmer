package warpsync

import (
	"context"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
	"github.com/iotaledger/goshimmer/packages/node/warpsync"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Warpsync"

var (
	// Plugin is the plugin instance of the gossip plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle          *tangle.Tangle
	WarpsyncMgr     *warpsync.Manager
	NotarizationMgr *notarization.Manager
	P2PMgr          *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(func(t *tangle.Tangle, p2pManager *p2p.Manager) *warpsync.Manager {
			return warpsync.NewManager(t, p2pManager, Plugin.Logger(), warpsync.WithConcurrency(Parameters.Concurrency), warpsync.WithBlockBatchSize(Parameters.BlockBatchSize))
		}); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	deps.NotarizationMgr.Events.SyncRange.Hook(event.NewClosure(func(event *notarization.SyncRangeEvent) {
		deps.WarpsyncMgr.Lock()
		defer deps.WarpsyncMgr.Unlock()

		Plugin.LogInfof("warpsyncing range %d-%d on chain %s", event.StartEI, event.EndEI, event.EndPrevEC.Base58())
		ctx, cancel := context.WithTimeout(context.Background(), Parameters.SyncRangeTimeOut)
		defer cancel()
		ecChain, validateErr := deps.WarpsyncMgr.ValidateBackwards(ctx, event.StartEI, event.EndEI, event.StartEC, event.EndPrevEC)
		if validateErr != nil {
			Plugin.LogWarnf("failed to validate range %d-%d: %s", event.StartEI, event.EndEI, validateErr)
			return
		}
		if syncRangeErr := deps.WarpsyncMgr.SyncRange(ctx, event.StartEI, event.EndEI, ecChain); syncRangeErr != nil {
			Plugin.LogWarnf("failed to sync range %d-%d: %s", event.StartEI, event.EndEI, syncRangeErr)
			return
		}
	}))
}

func start(ctx context.Context) {
	defer Plugin.LogInfo("Stopping " + PluginName + " ... done")
	<-ctx.Done()
	Plugin.LogInfo("Stopping " + PluginName + " ...")
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityWarpsync); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}
