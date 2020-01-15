package gossip

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/goshimmer/packages/autopeering/selection"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const name = "Gossip" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	configureGossip()
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(name, start, shutdown.ShutdownPriorityGossip); err != nil {
		log.Errorf("Failed to start as daemon: %s", err)
	}
}

func configureEvents() {
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		go func() {
			if err := mgr.DropNeighbor(ev.DroppedID); err != nil {
				log.Debugw("error dropping neighbor", "id", ev.DroppedID, "err", err)
			}
		}()
	}))
	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		go func() {
			if err := mgr.AddInbound(ev.Peer); err != nil {
				log.Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		go func() {
			if err := mgr.AddOutbound(ev.Peer); err != nil {
				log.Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	gossip.Events.NeighborAdded.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		log.Infof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	gossip.Events.NeighborRemoved.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("Neighbor removed: %s / %s", gossip.GetAddress(p), p.ID())
	}))

	// gossip transactions on solidification
	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		mgr.SendTransaction(tx.GetBytes())
	}))
	tangle.SetRequester(tangle.RequesterFunc(requestTransaction))
}
