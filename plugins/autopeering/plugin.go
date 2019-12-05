package autopeering

import (
	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/goshimmer/packages/gossip/neighbor"
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
	gossip.Events.DropNeighbor.Attach(events.NewClosure(func(peer *neighbor.Neighbor) {
		Selection.DropPeer(peer.Peer)
	}))

	// selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
	// 	log.Debug("neighbor removed: " + ev.DroppedID.String())
	// 	gossip.RemoveNeighbor(ev.DroppedID.String())
	// }))

	// selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
	// 	log.Debug("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
	// 	address, port, _ := net.SplitHostPort(ev.Services["gossip"].Address)
	// 	gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
	// }))

	// selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
	// 	log.Debug("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
	// 	address, port, _ := net.SplitHostPort(ev.Services["gossip"].Address)
	// 	gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
	// }))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Info("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
