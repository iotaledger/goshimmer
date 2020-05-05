package autopeering

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/hive.go/autopeering/discover"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

// PluginName is the name of the autopeering plugin.
const PluginName = "Autopeering"

var (
	// Plugin is the plugin instance of the autopeering plugin.
	Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)

	log *logger.Logger
)

func configure(*node.Plugin) {
	log = logger.NewLogger(PluginName)

	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(PluginName, start, shutdown.PriorityAutopeering); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	// assure that the autopeering is instantiated
	peerDisc := Discovery()
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
