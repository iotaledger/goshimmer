package autopeering

import (
	"net"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/peer/service"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/plugins/gossip"
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
	gossip.Events.RemoveNeighbor.Attach(events.NewClosure(func(peer *gossip.Neighbor) {
		if Selection != nil {
			Selection.DropPeer(peer.Peer)
		}
	}))

	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Info("neighbor removed: " + ev.DroppedID.String())
		gossip.RemoveNeighbor(ev.DroppedID.String())
	}))

	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Info("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
		log.Info("services: " + ev.Peer.Services().CreateRecord().String())
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
		}
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		log.Info("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
		log.Info("services: " + ev.Peer.Services().CreateRecord().String())
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			address, port, _ := net.SplitHostPort(ev.Peer.Services().Get(service.GossipKey).String())
			gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
		}
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
