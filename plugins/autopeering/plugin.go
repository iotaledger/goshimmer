package autopeering

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/discover"
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/selection"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
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
	if err := daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityAutopeering); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// notify the selection when a connection is closed or failed.
	gossip.Events.ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer) {
		Selection.DropPeer(p)
	}))
	gossip.Events.NeighborRemoved.Attach(events.NewClosure(func(p *peer.Peer) {
		Selection.DropPeer(p)
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	discover.Events.PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("Peering dropped: %s", ev.DroppedID)
	}))
}
