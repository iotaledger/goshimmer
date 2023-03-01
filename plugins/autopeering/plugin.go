package autopeering

import (
	"context"
	"net"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1/manamodels"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/runtime/event"
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
	AutopeeringConnMetric *UDPConnTraffic
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(func(event *node.InitEvent) {
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
	})
}

func configure(plugin *node.Plugin) {
	var err error

	// resolve the bind address
	localAddr, err = net.ResolveUDPAddr("udp", Parameters.BindAddress)
	if err != nil {
		Plugin.LogFatalfAndExitf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the peering service
	if err := deps.Local.UpdateService(service.PeeringKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin.LogFatalfAndExitf("could not update services: %s", err)
	}

	if deps.P2PMgr != nil {
		configureGossipIntegration(plugin)
	}
	configureEvents(plugin)
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityAutopeering); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureGossipIntegration(plugin *node.Plugin) {
	// assure that the Manager is instantiated
	mgr := deps.P2PMgr

	// link to the autopeering events
	deps.Selection.Events().Dropped.Hook(func(ev *selection.DroppedEvent) {
		if err := mgr.DropNeighbor(ev.DroppedID, p2p.NeighborsGroupAuto); err != nil {
			Plugin.Logger().Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
		}
	}, event.WithWorkerPool(plugin.WorkerPool))
	// We need to allocate synchronously the resources to accommodate incoming stream requests.
	deps.Selection.Events().IncomingPeering.Hook(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		if err := mgr.AddInbound(context.Background(), ev.Peer, p2p.NeighborsGroupAuto); err != nil {
			deps.Selection.RemoveNeighbor(ev.Peer.ID())
			Plugin.Logger().Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
		}
	})

	deps.Selection.Events().OutgoingPeering.Hook(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		if err := mgr.AddOutbound(context.Background(), ev.Peer, p2p.NeighborsGroupAuto); err != nil {
			deps.Selection.RemoveNeighbor(ev.Peer.ID())
			Plugin.Logger().Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
		}
	}, event.WithWorkerPool(plugin.WorkerPool))

	mgr.NeighborGroupEvents(p2p.NeighborsGroupAuto).NeighborRemoved.Hook(func(event *p2p.NeighborRemovedEvent) {
		deps.Selection.RemoveNeighbor(event.Neighbor.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func configureEvents(plugin *node.Plugin) {
	// log the peer discovery events
	deps.Discovery.Events().PeerDiscovered.Hook(func(ev *discover.PeerDiscoveredEvent) {
		Plugin.Logger().Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))
	deps.Discovery.Events().PeerDeleted.Hook(func(ev *discover.PeerDeletedEvent) {
		Plugin.Logger().Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}, event.WithWorkerPool(plugin.WorkerPool))

	// log the peer selection events
	deps.Selection.Events().SaltUpdated.Hook(func(ev *selection.SaltUpdatedEvent) {
		Plugin.Logger().Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}, event.WithWorkerPool(plugin.WorkerPool))
	deps.Selection.Events().OutgoingPeering.Hook(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}, event.WithWorkerPool(plugin.WorkerPool))
	deps.Selection.Events().IncomingPeering.Hook(func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}, event.WithWorkerPool(plugin.WorkerPool))
	deps.Selection.Events().Dropped.Hook(func(ev *selection.DroppedEvent) {
		Plugin.Logger().Infof("Peering dropped: %s", ev.DroppedID)
	}, event.WithWorkerPool(plugin.WorkerPool))
}

func start(ctx context.Context) {
	defer Plugin.Logger().Info("Stopping " + PluginName + " ... done")

	conn, err := net.ListenUDP(localAddr.Network(), localAddr)
	if err != nil {
		Plugin.Logger().Fatalf("Error listening: %v", err)
	}
	defer conn.Close()

	// use wrapped UDPConn to allow metrics collection
	deps.AutopeeringConnMetric.UDPConn = conn

	lPeer := deps.Local

	// start a server doing peerDisc and peering
	srv := server.Serve(lPeer, deps.AutopeeringConnMetric, Plugin.Logger().Named("srv"), deps.Discovery, deps.Selection)
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
