package gossip

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/configuration"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Gossip"

var (
	// plugin is the plugin instance of the gossip plugin.
	Plugin *node.Plugin
	once   sync.Once

	deps dependencies
)

type dependencies struct {
	dig.In

	Node      *configuration.Configuration
	Local     *peer.Local
	Tangle    *tangle.Tangle
	GossipMgr *gossip.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)

	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		fmt.Println("gossip provided")
		if err := dependencyinjection.Container.Provide(func(peerLocal *peer.Local, t *tangle.Tangle) *gossip.Manager {
			mgr := createManager(peerLocal, t)
			return mgr
		}); err != nil {
			panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	if err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	}); err != nil {
		plugin.LogError(err)
	}
	configureLogging()
	configureMessageLayer()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityGossip); err != nil {
		Plugin.Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureLogging() {
	// log the gossip events
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer, err error) {
		Plugin.LogInfof("Connection to neighbor %s / %s failed: %s", gossip.GetAddress(p), p.ID(), err)
	}))
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin.LogInfof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	deps.GossipMgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin.LogInfof("Neighbor removed: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
}

func configureMessageLayer() {
	// configure flow of incoming messages
	deps.GossipMgr.Events().MessageReceived.Attach(events.NewClosure(func(event *gossip.MessageReceivedEvent) {
		deps.Tangle.ProcessGossipMessage(event.Data, event.Peer)
	}))

	// configure flow of outgoing messages (gossip after ordering)
	deps.Tangle.Orderer.Events.MessageOrdered.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
			deps.GossipMgr.SendMessage(message.Bytes())
		})
	}))

	// request missing messages
	deps.Tangle.Requester.Events.SendRequest.Attach(events.NewClosure(func(sendRequest *tangle.SendRequestEvent) {
		deps.GossipMgr.RequestMessage(sendRequest.ID[:])
	}))
}
