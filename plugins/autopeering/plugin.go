package autopeering

import (
	"context"
	"net"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	gossipplugin "github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/webapi/mana"
)

// PluginName is the name of the peering plugin.
const PluginName = "AutoPeering"

var (
	// plugin is the plugin instance of the peering plugin.
	plugin *node.Plugin
	once   sync.Once

	localAddr   *net.UDPAddr
	manaEnabled bool
)

// Plugin gets the plugin instance.
func Plugin() *node.Plugin {
	once.Do(func() {
		plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)
	})
	return plugin
}

func configure(*node.Plugin) {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveUDPAddr("udp", Parameters.BindAddress)
	if err != nil {
		Plugin().LogFatalf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the peering service
	if err := local.GetInstance().UpdateService(service.PeeringKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin().LogFatalf("could not update services: %s", err)
	}

	manaEnabled = node.IsSkipped(mana.Plugin())

	if !node.IsSkipped(gossipplugin.Plugin()) {
		configureGossipIntegration()
	}
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityAutopeering); err != nil {
		Plugin().Panicf("Failed to start as daemon: %s", err)
	}
}

func configureGossipIntegration() {
	// assure that the Manager is instantiated
	mgr := gossipplugin.Manager()

	// link to the autopeering events
	peerSel := Selection()
	peerSel.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		go func() {
			if err := mgr.DropNeighbor(ev.DroppedID, gossip.NeighborsGroupAuto); err != nil {
				Plugin().Logger().Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
			}
		}()
	}))
	peerSel.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddInbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				Plugin().Logger().Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))
	peerSel.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddOutbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				Plugin().Logger().Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	// notify the autopeering on connection loss
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer, _ error) {
		peerSel.RemoveNeighbor(p.ID())
	}))
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		peerSel.RemoveNeighbor(n.ID())
	}))
}

func configureEvents() {
	// assure that the autopeering is instantiated
	peerDisc := discovery.Discovery()
	peerSel := Selection()

	// log the peer discovery events
	peerDisc.Events().PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		Plugin().LogInfof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	peerDisc.Events().PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		Plugin().LogInfof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	// log the peer selection events
	peerSel.Events().SaltUpdated.Attach(events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		Plugin().LogInfof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	peerSel.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin().LogInfof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	peerSel.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin().LogInfof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	peerSel.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		Plugin().LogInfof("Peering dropped: %s", ev.DroppedID)
	}))
}

func start(shutdownSignal <-chan struct{}) {
	defer Plugin().LogInfo("Stopping " + PluginName + " ... done")

	conn, err := net.ListenUDP(localAddr.Network(), localAddr)
	if err != nil {
		Plugin().LogFatalf("Error listening: %v", err)
	}
	defer conn.Close()

	Conn = &NetConnMetric{UDPConn: conn}
	lPeer := local.GetInstance()

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, Conn, Plugin().Logger().Named("srv"), discovery.Discovery(), Selection())
	defer srv.Close()

	// start the peer discovery on that connection
	discovery.Discovery().Start(srv)

	// start the neighbor selection process.
	Selection().Start(srv)

	Plugin().LogInfof("%s started: ID=%s Address=%s/%s", PluginName, lPeer.ID(), localAddr.String(), localAddr.Network())

	<-shutdownSignal

	Plugin().LogInfof("Stopping %s ...", PluginName)
	Selection().Close()

	discovery.Discovery().Close()

	lPeer.Database().Close()
}
