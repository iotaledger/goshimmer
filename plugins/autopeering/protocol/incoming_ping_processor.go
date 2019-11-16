package protocol

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/ping"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
)

func createIncomingPingProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(ping *ping.Ping) {
		log.Debugf("received ping from %s", ping.Issuer.String())

		knownpeers.INSTANCE.AddOrUpdate(ping.Issuer)
		for _, neighbor := range ping.Neighbors.GetPeers() {
			knownpeers.INSTANCE.AddOrUpdate(neighbor)
		}
	})
}
