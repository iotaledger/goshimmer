package udp

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network/udp"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "github.com/pkg/errors"
    "math"
    "net"
    "strconv"
)

var udpServer = udp.NewServer(int(math.Max(float64(request.MARSHALLED_TOTAL_SIZE), float64(response.MARSHALLED_TOTAL_SIZE))))

func ConfigureServer(plugin *node.Plugin) {
    Events.Error.Attach(func(ip net.IP, err error) {
        plugin.LogFailure("u" + err.Error())
    })

    udpServer.Events.ReceiveData.Attach(processReceivedData)
    udpServer.Events.Error.Attach(func(err error) {
        plugin.LogFailure("error in udp server: " + err.Error())
    })
    udpServer.Events.Start.Attach(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogSuccess("Starting UDP Server (port " + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ... done")
        } else {
            plugin.LogSuccess("Starting UDP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ... done")
        }
    })
    udpServer.Events.Shutdown.Attach(func() {
        plugin.LogSuccess("Stopping UDP Server ... done")
    })
}

func RunServer(plugin *node.Plugin) {
    daemon.BackgroundWorker(func() {
        if *parameters.ADDRESS.Value == "0.0.0.0" {
            plugin.LogInfo("Starting UDP Server (port " + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ...")
        } else {
            plugin.LogInfo("Starting UDP Server (" + *parameters.ADDRESS.Value + ":" + strconv.Itoa(*parameters.UDP_PORT.Value) + ") ...")
        }

        udpServer.Listen(*parameters.ADDRESS.Value, *parameters.UDP_PORT.Value)
    })
}

func ShutdownUDPServer(plugin *node.Plugin) {
    plugin.LogInfo("Stopping UDP Server ...")

    udpServer.Shutdown()
}

func processReceivedData(addr *net.UDPAddr, data []byte) {
    switch data[0] {
    case request.MARSHALLED_PACKET_HEADER:
        if peeringRequest, err := request.Unmarshal(data); err != nil {
            Events.Error.Trigger(addr.IP, err)
        } else {
            peeringRequest.Issuer.Address = addr.IP

            Events.ReceiveRequest.Trigger(peeringRequest)
        }
    case response.MARHSALLED_PACKET_HEADER:
        if peeringResponse, err := response.Unmarshal(data); err != nil {
            Events.Error.Trigger(addr.IP, err)
        } else {
            peeringResponse.Issuer.Address = addr.IP

            Events.ReceiveResponse.Trigger(peeringResponse)
        }
    default:
        Events.Error.Trigger(addr.IP, errors.New("invalid UDP peering packet from " + addr.IP.String()))
    }
}

