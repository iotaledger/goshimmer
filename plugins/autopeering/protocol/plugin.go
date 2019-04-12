package protocol

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/udp"
)

func Configure(plugin *node.Plugin) {
    incomingPingProcessor := createIncomingPingProcessor(plugin)
    incomingRequestProcessor := createIncomingRequestProcessor(plugin)
    incomingResponseProcessor := createIncomingResponseProcessor(plugin)
    errorHandler := createErrorHandler(plugin)

    udp.Events.ReceivePing.Attach(incomingPingProcessor)
    udp.Events.ReceiveRequest.Attach(incomingRequestProcessor)
    udp.Events.ReceiveResponse.Attach(incomingResponseProcessor)
    udp.Events.Error.Attach(errorHandler)

    tcp.Events.ReceivePing.Attach(incomingPingProcessor)
    tcp.Events.ReceiveRequest.Attach(incomingRequestProcessor)
    tcp.Events.ReceiveResponse.Attach(incomingResponseProcessor)
    tcp.Events.Error.Attach(errorHandler)
}

func Run(plugin *node.Plugin) {
    daemon.BackgroundWorker(createOutgoingRequestProcessor(plugin))
    daemon.BackgroundWorker(createOutgoingPingProcessor(plugin))
}
