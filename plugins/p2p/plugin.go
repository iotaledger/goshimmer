package p2p

import (
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	p2p2 "github.com/iotaledger/goshimmer/packages/network/p2p"
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
	P2PMgr *p2p2.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createManager); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	configureLogging()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityP2P); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureLogging() {
	// log the p2p events
	deps.P2PMgr.NeighborGroupEvents(p2p2.NeighborsGroupAuto).NeighborAdded.Attach(event.NewClosure(func(event *p2p2.NeighborAddedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor added: %s / %s", p2p2.GetAddress(n.Peer), n.ID())
	}))
	deps.P2PMgr.NeighborGroupEvents(p2p2.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p2.NeighborRemovedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor removed: %s / %s", p2p2.GetAddress(n.Peer), n.ID())
	}))
}
