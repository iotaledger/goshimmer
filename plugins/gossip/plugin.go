package gossip

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/node/gossip"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Gossip"

var (
	// Plugin is the plugin instance of the gossip plugin.
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Tangle    *tangleold.Tangle
	GossipMgr *gossip.Manager
	P2PMgr    *p2p.Manager
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
	configureBlockLayer()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityGossip); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureLogging() {
	// log the gossip events
	deps.Tangle.Requester.Events.RequestStarted.Attach(event.NewClosure(func(event *tangleold.RequestStartedEvent) {
		Plugin.LogDebugf("started to request missing Block with %s", event.BlockID)
	}))
	deps.Tangle.Requester.Events.RequestStopped.Attach(event.NewClosure(func(event *tangleold.RequestStoppedEvent) {
		Plugin.LogDebugf("stopped to request missing Block with %s", event.BlockID)
	}))
	deps.Tangle.Requester.Events.RequestFailed.Attach(event.NewClosure(func(event *tangleold.RequestFailedEvent) {
		Plugin.LogDebugf("failed to request missing Block with %s", event.BlockID)
	}))
}

func configureBlockLayer() {
	// configure flow of incoming blocks
	deps.GossipMgr.Events.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
		deps.Tangle.ProcessGossipBlock(event.Data, event.Peer)
	}))

	// configure flow of outgoing blocks (gossip upon dispatched blocks)
	deps.Tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *tangleold.BlockScheduledEvent) {
		deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangleold.Block) {
			deps.GossipMgr.SendBlock(lo.PanicOnErr(block.Bytes()))
		})
	}))

	// request missing blocks
	deps.Tangle.Requester.Events.RequestIssued.Attach(event.NewClosure(func(event *tangleold.RequestIssuedEvent) {
		id := event.BlockID
		Plugin.LogDebugf("requesting missing Block with %s", id)

		deps.GossipMgr.RequestBlock(id.Bytes())
	}))
}
