package autopeering

import (
	"context"
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/dependencyinjection"
	"github.com/iotaledger/hive.go/autopeering/discover"
)

// PluginName is the name of the autopeering plugin.
const PluginName = "Autopeering"

var (
	// plugin is the plugin instance of the autopeering plugin.
	Plugin *node.Plugin
	deps   dependencies

	log         *logger.Logger
	manaEnabled bool
)

type dependencies struct {
	dig.In

	Discovery *discover.Protocol
	Selection *selection.Protocol
	Local     *peer.Local
	GossipMgr *gossip.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)

	Plugin.Events.Init.Attach(events.NewClosure(func(*node.Plugin) {
		fmt.Println("provide peer local")

		if err := dependencyinjection.Container.Provide(func() *peer.Local {
			return local.ConfigureLocal()
		}); err != nil {
			Plugin.Panic(err)
		}
		if err := dependencyinjection.Container.Provide(func(localID *peer.Local) *discover.Protocol {
			return discovery.CreatePeerDisc(localID)
		}); err != nil {
			Plugin.Panic(err)
		}
		if err := dependencyinjection.Container.Provide(func(localID *peer.Local, nbrDiscover *discover.Protocol) *selection.Protocol {
			return createPeerSel(localID, nbrDiscover)
		}); err != nil {
			Plugin.Panic(err)
		}
		if err := dependencyinjection.Container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("autopeering")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)
	err := dependencyinjection.Container.Invoke(func(dep dependencies) {
		deps = dep
	})
	if err != nil {
		Plugin.LogError(err)
	}

	if Parameters.EnableGossipIntegration {
		configureGossipIntegration()
	}
	configureEvents()
}

func run(*node.Plugin) {
	plugins := node.GetPlugins()
	if manaPlugin, ok := plugins["mana"]; ok {
		if !node.IsSkipped(manaPlugin) {
			manaEnabled = true
		}
	}
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityAutopeering); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func configureGossipIntegration() {
	log.Info("Configuring autopeering to manage neighbors in the gossip layer")
	// assure that the Manager is instantiated
	mgr := deps.GossipMgr

	// link to the autopeering events
	deps.Selection.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		go func() {
			if err := mgr.DropNeighbor(ev.DroppedID, gossip.NeighborsGroupAuto); err != nil {
				log.Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
			}
		}()
	}))
	deps.Selection.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddInbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				log.Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))
	deps.Selection.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddOutbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				log.Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	// notify the autopeering on connection loss
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer, _ error) {
		deps.Selection.RemoveNeighbor(p.ID())
	}))
	mgr.NeighborsEvents(gossip.NeighborsGroupAuto).NeighborRemoved.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		deps.Selection.RemoveNeighbor(n.ID())
	}))
}

func configureEvents() {
	// log the peer discovery events
	deps.Discovery.Events().PeerDiscovered.Attach(events.NewClosure(func(ev *discover.DiscoveredEvent) {
		log.Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	deps.Discovery.Events().PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	// log the peer selection events
	deps.Selection.Events().SaltUpdated.Attach(events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	deps.Selection.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	deps.Selection.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("Peering dropped: %s", ev.DroppedID)
	}))
}
