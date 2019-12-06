package autopeering

import (
	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

var PLUGIN = node.NewPlugin("Auto Peering", node.Enabled, configure, run)
var log = logger.NewLogger("Autopeering")

func configure(plugin *node.Plugin) {
	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		close <- struct{}{}
	}))

	configureAP()
	configureLogging(plugin)
}

func run(plugin *node.Plugin) {
	go start()
}

func configureLogging(plugin *node.Plugin) {
	gossip.Events.DropNeighbor.Attach(events.NewClosure(func(ev *gossip.DropNeighborEvent) {
		log.Info("neighbor dropped: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
		if Selection != nil {
			Selection.DropPeer(ev.Peer)
		}
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
