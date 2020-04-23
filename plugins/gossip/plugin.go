package gossip

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/gossip"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
)

const name = "Gossip" // name of the plugin

var PLUGIN = node.NewPlugin(name, node.Enabled, configure, run)

func configure(*node.Plugin) {
	log = logger.NewLogger(name)

	configureGossip()
	configureEvents()
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker(name, start, shutdown.PriorityGossip); err != nil {
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
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddInbound(ev.Peer); err != nil {
				log.Debugw("error adding inbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))
	selection.Events.OutgoingPeering.Attach(events.NewClosure(func(ev *selection.PeeringEvent) {
		if !ev.Status {
			return // ignore rejected peering
		}
		go func() {
			if err := mgr.AddOutbound(ev.Peer); err != nil {
				log.Debugw("error adding outbound", "id", ev.Peer.ID(), "err", err)
			}
		}()
	}))

	gossip.Events.ConnectionFailed.Attach(events.NewClosure(func(p *peer.Peer, err error) {
		log.Infof("Connection to neighbor %s / %s failed: %s", gossip.GetAddress(p), p.ID(), err)
	}))
	gossip.Events.NeighborAdded.Attach(events.NewClosure(func(n *gossip.Neighbor) {
		log.Infof("Neighbor added: %s / %s", gossip.GetAddress(n.Peer), n.ID())
	}))
	gossip.Events.NeighborRemoved.Attach(events.NewClosure(func(p *peer.Peer) {
		log.Infof("Neighbor removed: %s / %s", gossip.GetAddress(p), p.ID())
	}))

	// configure flow of incoming messages
	gossip.Events.MessageReceived.Attach(events.NewClosure(func(event *gossip.MessageReceivedEvent) {
		messagelayer.MessageParser.Parse(event.Data, event.Peer)
	}))

	// configure flow of outgoing messages (gossip on solidification)
	messagelayer.Tangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessageMetadata.Release()
		cachedMessage.Consume(func(msg *message.Message) {
			mgr.SendMessage(msg.Bytes())
		})
	}))

	// request missing messages
	messagelayer.MessageRequester.Events.SendRequest.Attach(events.NewClosure(func(messageId message.Id) {
		mgr.RequestMessage(messageId[:])
	}))
}
