package autopeering

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	name     = "Autopeering" // name of the plugin
	logLevel = "info"
)

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var log = logger.NewLogger(name)

func configure(*node.Plugin) {
	configureEvents()
	configureAP()
}

func run(*node.Plugin) {
	daemon.BackgroundWorker(name, start)
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
