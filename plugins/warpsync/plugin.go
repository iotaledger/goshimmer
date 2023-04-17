package warpsync

import (
	"context"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/requester/warpsync"
	"github.com/iotaledger/hive.go/app/daemon"
)

// PluginName is the name of the warpsync plugin.
const PluginName = "Warpsync"

var (
	// Plugin is the plugin instance of the warpsync plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Protocol    *protocol.Protocol
	WarpsyncMgr *warpsync.Manager
	P2PMgr      *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(func(p *protocol.Protocol, p2pManager *p2p.Manager) *warpsync.Manager {
			// TODO: refactor when its ready
			// // TODO: use a different block loader function
			// loadBlockFunc := func(blockID models.BlockID) (*models.Block, error) {
			//	block, exists := p.Engine().Block(blockID)
			//	if !exists {
			//		return nil, errors.Errorf("block %s not found", blockID)
			//	}
			//	return block, nil
			// }
			// processBlockFunc := func(blk *models.Block, peer *peer.Peer) {
			//	p..Parser.Events.BlockParsed.Trigger(&tangleold.BlockParsedEvent{
			//		Block: blk,
			//		Peer:  peer,
			//	})
			// }
			// return warpsync.NewManager(p2pManager, loadBlockFunc, processBlockFunc, Plugin.Logger(), warpsync.WithConcurrency(Parameters.Concurrency), warpsync.WithBlockBatchSize(Parameters.BlockBatchSize))
			return nil
		}); err != nil {
			Plugin.Panic(err)
		}
	})
}

func configure(_ *node.Plugin) {
	// deps.NotarizationMgr.Events.SyncRange.Attach(event.NewClosure(func(event *notarization.SyncRangeEvent) {
	// 	ctx, cancel := context.WithTimeout(context.Background(), Parameters.SyncRangeTimeOut)
	// 	defer cancel()
	// 	if err := deps.WarpsyncMgr.WarpRange(ctx, event.StartEI, event.EndEI, event.StartEC, event.EndPrevEC); err != nil {
	// 		Plugin.LogWarn("failed to warpsync:", err)
	// 	}
	// }))
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
