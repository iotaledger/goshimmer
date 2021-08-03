package gossip

import (
	"sync"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Gossip"

var (
	// plugin is the plugin instance of the gossip plugin.
	plugin *node.Plugin
	once   sync.Once
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(*node.Plugin) {
	configureLogging()
	configureMessageLayer()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityGossip); err != nil {
		Plugin().Logger().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureLogging() {
	// assure that the Manager is instantiated
	mgr := Manager()

	// log the gossip events
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer, err error) {
		Plugin().LogInfof("Connection to neighbor %s / %s failed: %s", gossip.GetAddress(p), p.ID(), err)
	}))
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborAdded.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin().LogInfof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		Plugin().LogInfof("Neighbor removed: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
}

func configureMessageLayer() {
	// assure that the Manager is instantiated
	mgr := Manager()

	// configure flow of incoming messages
	mgr.Events().MessageReceived.Attach(events.NewClosure(func(event *gossip.MessageReceivedEvent) {
		messagelayer.Tangle().ProcessGossipMessage(event.Data, event.Peer)
	}))

	// configure flow of outgoing messages (gossip after ordering)
	messagelayer.Tangle().Orderer.Events.MessageOrdered.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
			mgr.SendMessage(message.Bytes())
		})
	}))

	// request missing messages
	messagelayer.Tangle().Requester.Events.SendRequest.Attach(events.NewClosure(func(sendRequest *tangle.SendRequestEvent) {
		mgr.RequestMessage(sendRequest.ID[:])
	}))
}
