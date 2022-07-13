package gossip

import (
	"github.com/iotaledger/hive.go/generics/lo"
	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/p2p"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
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

	Local     *peer.Local
	Tangle    *tangle.Tangle
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
	deps.P2PMgr.NeighborsEvents(p2p.NeighborsGroupAuto).NeighborAdded.Attach(event.NewClosure(func(event *p2p.NeighborAddedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor added: %s / %s", p2p.GetAddress(n.Peer), n.ID())
	}))
	deps.P2PMgr.NeighborsEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor removed: %s / %s", p2p.GetAddress(n.Peer), n.ID())
	}))
	deps.Tangle.Requester.Events.RequestStarted.Attach(event.NewClosure(func(event *tangle.RequestStartedEvent) {
		Plugin.LogDebugf("started to request missing Block with %s", event.BlockID)
	}))
	deps.Tangle.Requester.Events.RequestStopped.Attach(event.NewClosure(func(event *tangle.RequestStoppedEvent) {
		Plugin.LogDebugf("stopped to request missing Block with %s", event.BlockID)
	}))
	deps.Tangle.Requester.Events.RequestFailed.Attach(event.NewClosure(func(event *tangle.RequestFailedEvent) {
		Plugin.LogDebugf("failed to request missing Block with %s", event.BlockID)
	}))
}

func configureBlockLayer() {
	// configure flow of incoming blocks
	deps.GossipMgr.Events.BlockReceived.Attach(event.NewClosure(func(event *gossip.BlockReceivedEvent) {
		deps.Tangle.ProcessGossipBlock(event.Data, event.Peer)
	}))

	// configure flow of outgoing blocks (gossip upon dispatched blocks)
	deps.Tangle.Scheduler.Events.BlockScheduled.Attach(event.NewClosure(func(event *tangle.BlockScheduledEvent) {
		deps.Tangle.Storage.Block(event.BlockID).Consume(func(block *tangle.Block) {
			deps.GossipMgr.SendBlock(lo.PanicOnErr(block.Bytes()))
		})
	}))

	// request missing blocks
	deps.Tangle.Requester.Events.RequestIssued.Attach(event.NewClosure(func(event *tangle.RequestIssuedEvent) {
		id := event.BlockID
		Plugin.LogDebugf("requesting missing Block with %s", id)

		deps.GossipMgr.RequestBlock(id.Bytes())
	}))
}
