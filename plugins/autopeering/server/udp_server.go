package server

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network/udp"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "net"
    "strconv"
)

var udpServer = udp.NewServer()

func ConfigureUDPServer(plugin *node.Plugin) {
    Events.Error.Attach(func(ip net.IP, err error) {
        plugin.LogFailure(err.Error())
    })

    udpServer.Events.ReceiveData.Attach(processReceivedData)
    udpServer.Events.Error.Attach(func(err error) {
        plugin.LogFailure(err.Error())
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

func RunUDPServer(plugin *node.Plugin) {
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

    if peeringRequest, err := protocol.UnmarshalPeeringRequest(data); err != nil {
        Events.Error.Trigger(addr.IP, err)
    } else {
        Events.ReceivePeeringRequest.Trigger(addr.IP, peeringRequest)
    }
}

