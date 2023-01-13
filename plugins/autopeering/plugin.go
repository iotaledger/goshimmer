package autopeering

import (
	"context"
	"net"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/hive.go/core/autopeering/discover"
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/peer/service"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/autopeering/server"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"

	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
)

// PluginName is the name of the peering plugin.
const PluginName = "AutoPeering"

var (
	// Plugin is the plugin instance of the autopeering plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)

	localAddr *net.UDPAddr
)

type dependencies struct {
	dig.In

	Discovery             *discover.Protocol
	Selection             *selection.Protocol
	Local                 *peer.Local
	P2PMgr                *p2p.Manager                 `optional:"true"`
	ManaFunc              manamodels.ManaRetrievalFunc `optional:"true" name:"manaFunc"`
	AutoPeeringConnMetric *UDPConnTraffic
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(discovery.CreatePeerDisc); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(createPeerSel); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("autopeering")); err != nil {
			Plugin.Panic(err)
		}

		if err := event.Container.Provide(func() *UDPConnTraffic {
			return &UDPConnTraffic{}
		}); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(_ *node.Plugin) {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveUDPAddr("udp", Parameters.BindAddress)
	if err != nil {
		Plugin.LogFatalfAndExit("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the peering service
	if err := deps.Local.UpdateService(service.PeeringKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin.LogFatalfAndExit("could not update services: %s", err)
	}

	if deps.P2PMgr != nil {
		configureGossipIntegration()
	}
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityAutopeering); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureGossipIntegration() {
	// assure that the Manager is instantiated
	mgr := deps.P2PMgr

	// link to the autopeering events
	deps.Selection.Events().Dropped.Attach(event.NewClosure(func(ev *selection.DroppedEvent) {
		if err := mgr.DropNeighbor(ev.DroppedID, p2p.NeighborsGroupAuto); err != nil {
			Plugin.Logger().Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
		}
	}))
	// We need to allocate synchronously the resources to accommodate incoming stream requests.
	deps.Selection.Events().IncomingPeering.Hook(event.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		if err := mgr.AddInbound(context.Background(), ev.Peer, p2p.NeighborsGroupAuto); err != nil {
			deps.Selection.RemoveNeighbor(ev.Peer.ID())
			Plugin.Logger().Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
		}
	}))

	deps.Selection.Events().OutgoingPeering.Attach(event.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		if err := mgr.AddOutbound(context.Background(), ev.Peer, p2p.NeighborsGroupAuto); err != nil {
			deps.Selection.RemoveNeighbor(ev.Peer.ID())
			Plugin.Logger().Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
		}
	}))

	mgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Attach(event.NewClosure(func(event *p2p.NeighborRemovedEvent) {
		deps.Selection.RemoveNeighbor(event.Neighbor.ID())
	}))
}

func configureEvents() {
	// log the peer discovery events
	deps.Discovery.Events().PeerDiscovered.Attach(event.NewClosure(func(ev *discover.PeerDiscoveredEvent) {
		Plugin.Logger().Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	deps.Discovery.Events().PeerDeleted.Attach(event.NewClosure(func(ev *discover.PeerDeletedEvent) {
		Plugin.Logger().Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	// log the peer selection events
	deps.Selection.Events().SaltUpdated.Attach(event.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		Plugin.Logger().Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	deps.Selection.Events().OutgoingPeering.Attach(event.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().IncomingPeering.Attach(event.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().Dropped.Attach(event.NewClosure(func(ev *selection.DroppedEvent) {
		Plugin.Logger().Infof("Peering dropped: %s", ev.DroppedID)
	}))
}

func start(ctx context.Context) {
	defer Plugin.Logger().Info("Stopping " + PluginName + " ... done")

	conn, err := net.ListenUDP(localAddr.Network(), localAddr)
	if err != nil {
		Plugin.Logger().Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	// use wrapped UDPConn to allow metrics collection
	deps.AutoPeeringConnMetric.UDPConn = conn

	lPeer := deps.Local

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, deps.AutoPeeringConnMetric, Plugin.Logger().Named("srv"), deps.Discovery, deps.Selection)
	defer srv.Close()

	// start the peer discovery on that connection
	deps.Discovery.Start(srv)

	// start the neighbor selection process.
	deps.Selection.Start(srv)

	Plugin.Logger().Infof("%s started: ID=%s Address=%s/%s", PluginName, lPeer.ID(), localAddr.String(), localAddr.Network())

	<-ctx.Done()

	Plugin.Logger().Infof("Stopping %s ...", PluginName)
	deps.Selection.Close()

	deps.Discovery.Close()

	lPeer.Database().Close()
}
