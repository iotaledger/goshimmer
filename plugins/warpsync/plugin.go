package warpsync

import (
	"context"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"
	"github.com/iotaledger/goshimmer/packages/node/warpsync"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
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

	Tangle          *tangleold.Tangle
	WarpsyncMgr     *warpsync.Manager
	NotarizationMgr *notarization.Manager
	P2PMgr          *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Disabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(func(t *tangleold.Tangle, p2pManager *p2p.Manager) *warpsync.Manager {
			// TODO: use a different block loader function
			loadBlockFunc := func(blockID tangleold.BlockID) (*tangleold.Block, error) {
				cachedBlock := t.Storage.Block(blockID)
				defer cachedBlock.Release()
				block, exists := cachedBlock.Unwrap()
				if !exists {
					return nil, errors.Errorf("block %s not found", blockID)
				}
				return block, nil
			}
			processBlockFunc := func(blk *tangleold.Block, peer *peer.Peer) {
				t.Parser.Events.BlockParsed.Trigger(&tangleold.BlockParsedEvent{
					Block: blk,
					Peer:  peer,
				})
			}
			return warpsync.NewManager(p2pManager, loadBlockFunc, processBlockFunc, Plugin.Logger(), warpsync.WithConcurrency(Parameters.Concurrency), warpsync.WithBlockBatchSize(Parameters.BlockBatchSize))
		}); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	deps.NotarizationMgr.Events.SyncRange.Attach(event.NewClosure(func(event *notarization.SyncRangeEvent) {
		ctx, cancel := context.WithTimeout(context.Background(), Parameters.SyncRangeTimeOut)
		defer cancel()
		if err := deps.WarpsyncMgr.WarpRange(ctx, event.StartEI, event.EndEI, event.StartEC, event.EndPrevEC); err != nil {
			Plugin.LogWarn("failed to warpsync:", err)
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
