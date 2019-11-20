package autopeering

import (
	"net"

	"github.com/iotaledger/autopeering-sim/discover"
	"github.com/iotaledger/autopeering-sim/selection"
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/hive.go/events"
)

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
		Selection.DropPeer(peer.Peer)
	}))

	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		plugin.LogDebug("neighbor removed: " + ev.DroppedID.String())
		gossip.RemoveNeighbor(ev.DroppedID.String())
	}))

	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		plugin.LogDebug("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
		address, _, _ := net.SplitHostPort(ev.Peer.Address())
		port := ev.Services["gossip"].Address
		gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		plugin.LogDebug("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
		address, _, _ := net.SplitHostPort(ev.Peer.Address())
		port := ev.Services["gossip"].Address
		gossip.AddNeighbor(gossip.NewNeighbor(ev.Peer, address, port))
	}))

	discover.Events.PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		plugin.LogInfo("new peer discovered: " + ev.Peer.Address() + " / " + ev.Peer.ID().String())
	}))
}
