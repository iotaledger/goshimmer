package protocol

import (
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
)

func createIncomingPingProcessor(plugin *node.Plugin) func(p *ping.Ping) {
    return func(p *ping.Ping) {
        if p.Issuer.Conn != nil {
            plugin.LogDebug("received TCP ping from " + p.Issuer.String())
        } else {
            plugin.LogDebug("received UDP ping from " + p.Issuer.String())
        }

        Events.DiscoverPeer.Trigger(p.Issuer)
    }
}
