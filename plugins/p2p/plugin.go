package p2p

import (
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/runtime/event"
)

// PluginName is the name of the p2p plugin.
const PluginName = "P2P"

var (
	// Plugin is the plugin instance of the p2p plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Local  *peer.Local
	P2PMgr *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
		if err := event.Container.Provide(createManager); err != nil {
			Plugin.Panic(err)
		}
	})
}

func configure(plugin *node.Plugin) {
	// log the p2p events
	deps.P2PMgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborAdded.Hook(func(event *p2p.NeighborAddedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor added: %s / %s", p2p.GetAddress(n.Peer), n.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))

	deps.P2PMgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Hook(func(event *p2p.NeighborRemovedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor removed: %s / %s", p2p.GetAddress(n.Peer), n.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityP2P); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}
