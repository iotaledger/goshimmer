package gossip

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/network/tcp"
    "github.com/iotaledger/goshimmer/packages/node"
    "strconv"
)

var TCPServer = tcp.NewServer()

func configureServer(plugin *node.Plugin) {
    TCPServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
        newProtocol(conn).init()
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
