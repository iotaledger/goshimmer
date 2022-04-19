package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
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

	Node      *configuration.Configuration
	Local     *peer.Local
	Tangle    *tangle.Tangle
	GossipMgr *gossip.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Attach(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(createManager); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	configureLogging()
	configureMessageLayer()
}

func run(plugin *node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityGossip); err != nil {
		plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureLogging() {
	// log the gossip events
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(event.NewClosure(func(event *gossip.NeighborAddedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *gossip.NeighborRemovedEvent) {
		n := event.Neighbor
		Plugin.LogInfof("Neighbor removed: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	deps.Tangle.Requester.Events.RequestStarted.Attach(event.NewClosure(func(event *tangle.RequestStartedEvent) {
		Plugin.LogDebugf("started to request missing Message with %s", event.MessageID)
	}))
	deps.Tangle.Requester.Events.RequestStopped.Attach(event.NewClosure(func(event *tangle.RequestStoppedEvent) {
		Plugin.LogDebugf("stopped to request missing Message with %s", event.MessageID)
	}))
	deps.Tangle.Requester.Events.RequestFailed.Attach(event.NewClosure(func(event *tangle.RequestFailedEvent) {
		Plugin.LogDebugf("failed to request missing Message with %s", event.MessageID)
	}))
}

func configureMessageLayer() {
	// configure flow of incoming messages
	deps.GossipMgr.Events.MessageReceived.Attach(event.NewClosure(func(event *gossip.MessageReceivedEvent) {
		deps.Tangle.ProcessGossipMessage(event.Data, event.Peer)
	}))

	// configure flow of outgoing messages (gossip upon dispatched messages)
	deps.Tangle.Dispatcher.Events.MessageDispatched.Attach(event.NewClosure(func(event *tangle.MessageDispatchedEvent) {
		deps.Tangle.Storage.Message(event.MessageID).Consume(func(message *tangle.Message) {
			deps.GossipMgr.SendMessage(message.Bytes())
		})
	}))

	// request missing messages
	deps.Tangle.Requester.Events.RequestIssued.Attach(event.NewClosure(func(event *tangle.RequestIssuedEvent) {
		id := event.MessageID
		Plugin.LogDebugf("requesting missing Message with %s", id)

		deps.GossipMgr.RequestMessage(id[:])
	}))
}
