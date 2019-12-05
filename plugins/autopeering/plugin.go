package autopeering

import (
	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/gossip/neighbor"
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

	configureLogging(plugin)
}

func run(plugin *node.Plugin) {
	go start()
}

func configureLogging(plugin *node.Plugin) {
	gossip.Event.DropNeighbor.Attach(events.NewClosure(func(peer *neighbor.Neighbor) {
		if Selection != nil {
			Selection.DropPeer(peer.Peer)
		}
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
