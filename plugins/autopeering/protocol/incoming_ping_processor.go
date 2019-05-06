package protocol

import (
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
)

func createIncomingPingProcessor(plugin *node.Plugin) *events.Closure {
    return events.NewClosure(func(ping *ping.Ping) {
        plugin.LogDebug("received ping from " + ping.Issuer.String())

        knownpeers.INSTANCE.AddOrUpdate(ping.Issuer)
        for _, neighbor := range ping.Neighbors {
            knownpeers.INSTANCE.AddOrUpdate(neighbor)
        }
    })
}
