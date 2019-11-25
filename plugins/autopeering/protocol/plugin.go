package protocol

import (
	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
	"github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
	"github.com/iotaledger/goshimmer/plugins/autopeering/server/udp"
	"github.com/iotaledger/hive.go/parameter"
)

func Configure(plugin *node.Plugin) {
	errorHandler := createErrorHandler(plugin)

	udp.Events.ReceiveDrop.Attach(createIncomingDropProcessor(plugin))
	udp.Events.ReceivePing.Attach(createIncomingPingProcessor(plugin))
	udp.Events.Error.Attach(errorHandler)

	tcp.Events.ReceiveRequest.Attach(createIncomingRequestProcessor(plugin))
	tcp.Events.ReceiveResponse.Attach(createIncomingResponseProcessor(plugin))
	tcp.Events.Error.Attach(errorHandler)
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Autopeering Chosen Neighbor Dropper", createChosenNeighborDropper(plugin))
	daemon.BackgroundWorker("Autopeering Accepted Neighbor Dropper", createAcceptedNeighborDropper(plugin))

	if parameter.NodeConfig.GetBool(parameters.CFG_SEND_REQUESTS) {
		daemon.BackgroundWorker("Autopeering Outgoing Request Processor", createOutgoingRequestProcessor(plugin))
	}

	daemon.BackgroundWorker("Autopeering Outgoing Ping Processor", createOutgoingPingProcessor(plugin))
}
