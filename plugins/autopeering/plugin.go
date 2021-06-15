package autopeering

import (
	"context"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering/discovery"
	gossipplugin "github.com/iotaledger/goshimmer/plugins/gossip"
)

// PluginName is the name of the autopeering plugin.
const PluginName = "Autopeering"

var (
	// plugin is the plugin instance of the autopeering plugin.
	plugin *node.Plugin
	once   sync.Once

	log         *logger.Logger
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
	log = logger.NewLogger(PluginName)
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
	mgr := gossipplugin.Manager()

	// link to the autopeering events
	peerSel := Selection()
	peerSel.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		go func() {
			if err := mgr.DropNeighbor(ev.DroppedID, gossip.NeighborsGroupAuto); err != nil {
				log.Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
			}
		}()
	}))
	peerSel.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddInbound(context.Background(), ev.Peer, gossip.NeighborsGroupAuto); err != nil {
				log.Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))
	peerSel.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
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
		log.Infof("Discovered: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))
	peerDisc.Events().PeerDeleted.Attach(events.NewClosure(func(ev *discover.DeletedEvent) {
		log.Infof("Removed offline: %s / %s", ev.Peer.Address(), ev.Peer.ID())
	}))

	// log the peer selection events
	peerSel.Events().SaltUpdated.Attach(events.NewClosure(func(ev *selection.SaltUpdatedEvent) {
		log.Infof("Salt updated; expires=%s", ev.Public.GetExpiration().Format(time.RFC822))
	}))
	peerSel.Events().OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering chosen: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	peerSel.Events().IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if ev.Status {
			log.Infof("Peering accepted: %s / %s", ev.Peer.Address(), ev.Peer.ID())
		}
	}))
	peerSel.Events().Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Infof("Peering dropped: %s", ev.DroppedID)
	}))
}
