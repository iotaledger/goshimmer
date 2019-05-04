package autopeering

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/acceptedneighbors"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
)

var PLUGIN = node.NewPlugin("Auto Peering", configure, run)

func configure(plugin *node.Plugin) {
    saltmanager.Configure(plugin)
    instances.Configure(plugin)
    server.Configure(plugin)
    protocol.Configure(plugin)

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
    acceptedneighbors.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogSuccess("neighbor added: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))
    acceptedneighbors.INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogSuccess("neighbor removed: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))

    chosenneighbors.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogSuccess("neighbor added: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))
    chosenneighbors.INSTANCE.Events.Remove.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogSuccess("neighbor removed: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))

    knownpeers.INSTANCE.Events.Add.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogInfo("peer discovered: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))
    knownpeers.INSTANCE.Events.Update.Attach(events.NewClosure(func(p *peer.Peer) {
        plugin.LogDebug("peer updated: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
    }))
}
