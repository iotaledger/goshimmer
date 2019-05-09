package gossip

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/network/tcp"
    "github.com/iotaledger/goshimmer/packages/node"
    "net"
    "strconv"
)

var TCPServer = tcp.NewServer()

func configureServer(plugin *node.Plugin) {
    TCPServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
        neighbor := &Peer{
            Address: conn.RemoteAddr().(*net.TCPAddr).IP,
        }

        protocol := newProtocol(neighbor)

        var onClose, onReceiveData *events.Closure

        onReceiveData = events.NewClosure(func(data []byte) {
            protocol.parseData(data)
        })
        onClose = events.NewClosure(func() {
            conn.Events.ReceiveData.Detach(onReceiveData)
            conn.Events.Close.Detach(onClose)
        })

        conn.Events.ReceiveData.Attach(onReceiveData)
        conn.Events.Close.Attach(onClose)

        go conn.Read(make([]byte, 1000))
    }))

    daemon.Events.Shutdown.Attach(events.NewClosure(func() {
        plugin.LogInfo("Stopping TCP Server ...")

        TCPServer.Shutdown()
    }))
}

func runServer(plugin *node.Plugin) {
    plugin.LogInfo("Starting TCP Server (port " + strconv.Itoa(*PORT.Value) + ") ...")

    daemon.BackgroundWorker(func() {
        plugin.LogSuccess("Starting TCP Server (port " + strconv.Itoa(*PORT.Value) + ") ... done")

        TCPServer.Listen(*PORT.Value)

        plugin.LogSuccess("Stopping TCP Server ... done")
    })
}
