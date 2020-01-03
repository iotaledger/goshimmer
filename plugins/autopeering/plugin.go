package autopeering

import (
	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// NETWORK defines the network type used for the autopeering.
const NETWORK = "udp"

var PLUGIN = node.NewPlugin("Autopeering", node.Enabled, configure, run)

var log = logger.NewLogger("Autopeering")

func configure(*node.Plugin) {
	configureEvents()
	configureLocal()
	configureAP()
}

func run(*node.Plugin) {
	daemon.BackgroundWorker("Autopeering", start)
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
