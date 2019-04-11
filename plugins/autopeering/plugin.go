package autopeering

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
)

func configure(plugin *node.Plugin) {
    server.Configure(plugin)
    protocol.Configure(plugin)

    daemon.Events.Shutdown.Attach(func() {
        server.Shutdown(plugin)
    })

    protocol.Events.IncomingRequestAccepted.Attach(func(req *request.Request) {
        if neighbormanager.ACCEPTED_NEIGHBORS.AddOrUpdate(req.Issuer) {
            plugin.LogSuccess("new neighbor accepted: " + req.Issuer.Address.String() + " / " + req.Issuer.Identity.StringIdentifier)
        }
    })

    protocol.Events.OutgoingRequestAccepted.Attach(func(res *response.Response) {
        if neighbormanager.CHOSEN_NEIGHBORS.AddOrUpdate(res.Issuer) {
            plugin.LogSuccess("new neighbor chosen: " + res.Issuer.Address.String() + " / " + res.Issuer.Identity.StringIdentifier)
        }
    })

    protocol.Events.DiscoverPeer.Attach(func(p *peer.Peer) {
        if peermanager.KNOWN_PEERS.AddOrUpdate(p) {
            plugin.LogInfo("new peer detected: " + p.Address.String() + " / " + p.Identity.StringIdentifier)
        }
    })
}

func run(plugin *node.Plugin) {
    server.Run(plugin)
    protocol.Run(plugin)
}

var PLUGIN = node.NewPlugin("Auto Peering", configure, run)
