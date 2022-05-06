package autopeering

import (
	"context"
	"net"
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/autopeering/server"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/mana"
	net2 "github.com/iotaledger/goshimmer/packages/net"
	"github.com/iotaledger/goshimmer/packages/shutdown"
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
	GossipMgr             *gossip.Manager        `optional:"true"`
	ManaFunc              mana.ManaRetrievalFunc `optional:"true" name:"manaFunc"`
	AutoPeeringConnMetric *net2.ConnMetric
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Hook(event.NewClosure[*node.InitEvent](func(event *node.InitEvent) {
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

		if err := event.Container.Provide(func() *net2.ConnMetric {
			return &net2.ConnMetric{}
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
		Plugin.LogFatalf("bind address '%s' is invalid: %s", Parameters.BindAddress, err)
	}

	// announce the peering service
	if err := deps.Local.UpdateService(service.PeeringKey, localAddr.Network(), localAddr.Port); err != nil {
		Plugin.LogFatalf("could not update services: %s", err)
	}

	if deps.GossipMgr != nil {
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
	mgr := deps.GossipMgr

	// link to the autopeering events
	deps.Selection.Events().Dropped.Hook(event.NewClosure[*selection.DroppedEvent](func(ev *selection.DroppedEvent) {
		go func() {
			if err := mgr.DropNeighbor(ev.DroppedID, gossip.NeighborsGroupAuto); err != nil {
				Plugin.Logger().Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
			}
		}()
	}))
	deps.Selection.Events().IncomingPeering.Hook(event.NewClosure[*selection.PeeringEvent](func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddInbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				deps.Selection.RemoveNeighbor(ev.Peer.ID())
				Plugin.Logger().Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	deps.Selection.Events().OutgoingPeering.Hook(event.NewClosure[*selection.PeeringEvent](func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddOutbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				deps.Selection.RemoveNeighbor(ev.Peer.ID())
				Plugin.Logger().Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		deps.Selection.RemoveNeighbor(n.ID())
	}))
}

func configureEvents() {
	// log the peer discovery events
	deps.Discovery.Events().PeerDiscovered.Hook(event.NewClosure[*discover.PeerDiscoveredEvent](func(ev *discover.PeerDiscoveredEvent) {
		Plugin.Logger().Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	deps.Discovery.Events().PeerDeleted.Hook(event.NewClosure[*discover.PeerDeletedEvent](func(ev *discover.PeerDeletedEvent) {
		Plugin.Logger().Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	// log the peer selection events
	deps.Selection.Events().SaltUpdated.Hook(event.NewClosure[*selection.SaltUpdatedEvent](func(ev *selection.SaltUpdatedEvent) {
		Plugin.Logger().Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	deps.Selection.Events().OutgoingPeering.Hook(event.NewClosure[*selection.PeeringEvent](func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().IncomingPeering.Hook(event.NewClosure[*selection.PeeringEvent](func(ev *selection.PeeringEvent) {
		if ev.Status {
			Plugin.Logger().Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().Dropped.Hook(event.NewClosure[*selection.DroppedEvent](func(ev *selection.DroppedEvent) {
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

	// ideally this would happen during provide()
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
