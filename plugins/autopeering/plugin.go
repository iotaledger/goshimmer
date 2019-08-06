package autopeering

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/peerstorage"
	"github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
	"github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
	"github.com/iotaledger/goshimmer/plugins/autopeering/server"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/goshimmer/plugins/gossip"
)

var PLUGIN = node.NewPlugin("Auto Peering", node.Enabled, configure, run)

func configure(plugin *node.Plugin) {
	saltmanager.Configure(plugin)
	instances.Configure(plugin)
	server.Configure(plugin)
	protocol.Configure(plugin)
	peerstorage.Configure(plugin)

	daemon.Events.Shutdown.Attach(events.NewClosure(func() {
		server.Shutdown(plugin)
	}))

	configureLogging(plugin)
}

func run(plugin *node.Plugin) {
	instances.Run(plugin)
	server.Run(plugin)
	protocol.Run(plugin)
}

func configureLogging(plugin *node.Plugin) {
	gossip.Events.RemoveNeighbor.Attach(events.NewClosure(func(peer *gossip.Neighbor) {
		chosenneighbors.INSTANCE.Remove(peer.GetIdentity().StringIdentifier)
		acceptedneighbors.INSTANCE.Remove(peer.GetIdentity().StringIdentifier)
	}))

	acceptedneighbors.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogDebug("accepted neighbor added: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		gossip.AddNeighbor(gossip.NewNeighbor(p.GetIdentity(), p.GetAddress(), p.GetGossipPort()))
	}))
	acceptedneighbors.INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogDebug("accepted neighbor removed: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		gossip.RemoveNeighbor(p.GetIdentity().StringIdentifier)
	}))

	chosenneighbors.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogDebug("chosen neighbor added: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		gossip.AddNeighbor(gossip.NewNeighbor(p.GetIdentity(), p.GetAddress(), p.GetGossipPort()))
	}))
	chosenneighbors.INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogDebug("chosen neighbor removed: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		gossip.RemoveNeighbor(p.GetIdentity().StringIdentifier)
	}))

	knownpeers.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogInfo("new peer discovered: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		if _, exists := gossip.GetNeighbor(p.GetIdentity().StringIdentifier); exists {
			gossip.AddNeighbor(gossip.NewNeighbor(p.GetIdentity(), p.GetAddress(), p.GetGossipPort()))
		}
	}))
	knownpeers.INSTANCE.Events.Update.Attach(events.NewClosure(func(p *peer.Peer) {
		plugin.LogDebug("peer updated: " + p.GetAddress().String() + " / " + p.GetIdentity().StringIdentifier)

		if _, exists := gossip.GetNeighbor(p.GetIdentity().StringIdentifier); exists {
			gossip.AddNeighbor(gossip.NewNeighbor(p.GetIdentity(), p.GetAddress(), p.GetGossipPort()))
		}
	}))
}
