package protocol

import (
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
	"github.com/iotaledger/hive.go/events"
)

func createIncomingPingProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(ping *ping.Ping) {
		plugin.LogDebug("received ping from " + ping.Issuer.String())

		knownpeers.INSTANCE.AddOrUpdate(ping.Issuer)
		for _, neighbor := range ping.Neighbors.GetPeers() {
			knownpeers.INSTANCE.AddOrUpdate(neighbor)
		}
	})
}
