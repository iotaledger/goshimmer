package gossip

import (
	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/selection"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/model/value_transaction"
	"github.com/iotaledger/goshimmer/packages/typeutils"
	"github.com/iotaledger/goshimmer/plugins/tangle"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
)

const (
	name     = "Gossip" // name of the plugin
	logLevel = "info"
)

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

var log = logger.NewLogger(name)

func configure(*node.Plugin) {
	configureGossip()
	configureEvents()
}

func run(*node.Plugin) {
	daemon.BackgroundWorker(name, start)
}

func configureEvents() {
	selection.Events.Dropped.Attach(events.NewClosure(func(ev *selection.DroppedEvent) {
		log.Info("neighbor removed: " + ev.DroppedID.String())
		go mgr.DropNeighbor(ev.DroppedID)
	}))

	selection.Events.IncomingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Info("accepted neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			go mgr.AddInbound(ev.Peer)
		}
	}))

	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		gossipService := ev.Peer.Services().Get(service.GossipKey)
		if gossipService != nil {
			log.Info("chosen neighbor added: " + ev.Peer.Address() + " / " + ev.Peer.String())
			go mgr.AddOutbound(ev.Peer)
		}
	}))

	tangle.Events.TransactionSolid.Attach(events.NewClosure(func(tx *value_transaction.ValueTransaction) {
		log.Debugf("gossip solid tx: hash=%s", tx.GetHash())
		go mgr.SendTransaction(tx.GetBytes())
	}))

	gossip.Events.RequestTransaction.Attach(events.NewClosure(func(ev *gossip.RequestTransactionEvent) {
		log.Debugf("gossip tx request: hash=%s", ev.Hash)
		go mgr.RequestTransaction(typeutils.StringToBytes(ev.Hash))
	}))
}
