package protocol

import (
	"github.com/iotaledger/goshimmer/packages/events"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/chosenneighbors"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
)

func createIncomingResponseProcessor(plugin *node.Plugin) *events.Closure {
	return events.NewClosure(func(peeringResponse *response.Response) {
		go processIncomingResponse(plugin, peeringResponse)
	})
}

func processIncomingResponse(plugin *node.Plugin, peeringResponse *response.Response) {
	plugin.LogDebug("received peering response from " + peeringResponse.Issuer.String())

	_ = peeringResponse.Issuer.Conn.Close()

	knownpeers.INSTANCE.AddOrUpdate(peeringResponse.Issuer)
	for _, peer := range peeringResponse.Peers {
		knownpeers.INSTANCE.AddOrUpdate(peer)
	}

	if peeringResponse.Type == response.TYPE_ACCEPT {
		defer chosenneighbors.INSTANCE.Lock()()

		chosenneighbors.INSTANCE.AddOrUpdate(peeringResponse.Issuer, false)
	}
}
