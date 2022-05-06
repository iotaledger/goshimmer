package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
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

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
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
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin.LogInfof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin.LogInfof("Neighbor removed: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	deps.Tangle.Requester.Events.RequestStarted.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Plugin.LogDebugf("started to request missing Message with %s", messageID)
	}))
	deps.Tangle.Requester.Events.RequestStopped.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Plugin.LogDebugf("stopped to request missing Message with %s", messageID)
	}))
	deps.Tangle.Requester.Events.RequestFailed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		Plugin.LogDebugf("failed to request missing Message with %s", messageID)
	}))
}

func configureMessageLayer() {
	// configure flow of incoming messages
	deps.GossipMgr.Events().MessageReceived.Attach(events.NewClosure(func(event *gossip.MessageReceivedEvent) {
		deps.Tangle.ProcessGossipMessage(event.Data, event.Peer)
	}))

	// configure flow of outgoing messages (gossip upon dispatched messages)
	deps.Tangle.Dispatcher.Events.MessageDispatched.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			deps.GossipMgr.SendMessage(message.Bytes())
		})
	}))

	// request missing messages
	deps.Tangle.Requester.Events.RequestIssued.Attach(events.NewClosure(func(sendRequest *tangle.SendRequestEvent) {
		Plugin.LogDebugf("requesting missing Message with %s", sendRequest.ID)

		deps.GossipMgr.RequestMessage(sendRequest.ID[:])
	}))
}
