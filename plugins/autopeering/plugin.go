package autopeering

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const name = "Autopeering" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	configureEvents()
	configureAP()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(name, start); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	gossip.Events.NeighborDropped.Attach(events.NewClosure(func(ev *gossip.NeighborDroppedEvent) {
		log.Info("neighbor dropped: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
		if Selection != nil {
			Selection.DropPeer(ev.Peer)
		}
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
