package server

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/network/tcp"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "math"
    "net"
    "strconv"
)

var tcpServer = tcp.NewServer()

func ConfigureTCPServer(plugin *node.Plugin) {
    tcpServer.Events.Connect.Attach(func(peer network.Connection) {
        receivedData := make([]byte, protocol.PEERING_REQUEST_MARSHALLED_TOTAL_SIZE)

        peer.SetTimeout(TCP_IDLE_TIMEOUT)

        offset := 0
        peer.OnReceiveData(func(data []byte) {
            remainingCapacity := int(math.Min(float64(protocol.PEERING_REQUEST_MARSHALLED_TOTAL_SIZE- offset), float64(len(data))))

            copy(receivedData[offset:], data[:remainingCapacity])
            offset += len(data)

            if offset >= protocol.PEERING_REQUEST_MARSHALLED_TOTAL_SIZE {
                if peeringRequest, err := protocol.UnmarshalPeeringRequest(receivedData); err != nil {
                    Events.Error.Trigger(peer.GetConnection().RemoteAddr().(*net.TCPAddr).IP, err)
                } else {
                    Events.ReceiveTCPPeeringRequest.Trigger(peer, peeringRequest)
                }
            }
        })

        go peer.HandleConnection()
    })

    tcpServer.Events.Error.Attach(func(err error) {
        plugin.LogFailure(err.Error())
    })
    tcpServer.Events.Start.Attach(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogSuccess("Starting TCP Server (port " + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ... done")
        } else {
            plugin.LogSuccess("Starting TCP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ... done")
        }
    })
    tcpServer.Events.Shutdown.Attach(func() {
        plugin.LogSuccess("Stopping TCP Server ... done")
    })
}

func RunTCPServer(plugin *node.Plugin) {
    daemon.BackgroundWorker(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogInfo("Starting TCP Server (port " + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ...")
        } else {
            plugin.LogInfo("Starting TCP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ...")
        }

        tcpServer.Listen(*parameters.UDP_PORT.Value)
    })
}

func ShutdownTCPServer(plugin *node.Plugin) {
    plugin.LogInfo("Stopping TCP Server ...")

    tcpServer.Shutdown()
}
