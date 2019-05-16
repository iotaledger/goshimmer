package gossip

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/errors"
    "github.com/iotaledger/goshimmer/packages/events"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/network/tcp"
    "github.com/iotaledger/goshimmer/packages/node"
    "strconv"
)

var TCPServer = tcp.NewServer()

func configureServer(plugin *node.Plugin) {
    TCPServer.Events.Connect.Attach(events.NewClosure(func(conn *network.ManagedConnection) {
        protocol := newProtocol(conn)

        // print protocol errors
        protocol.Events.Error.Attach(events.NewClosure(func(err errors.IdentifiableError) {
            plugin.LogFailure(err.Error())
        }))

        // store connection in neighbor if its a neighbor calling
        protocol.Events.ReceiveIdentification.Attach(events.NewClosure(func(identity *identity.Identity) {
            if protocol.Neighbor != nil {
                protocol.Neighbor.acceptedConnMutex.Lock()
                if protocol.Neighbor.AcceptedConn == nil {
                    protocol.Neighbor.AcceptedConn = protocol.Conn

                    protocol.Neighbor.AcceptedConn.Events.Close.Attach(events.NewClosure(func() {
                        protocol.Neighbor.acceptedConnMutex.Lock()
                        defer protocol.Neighbor.acceptedConnMutex.Unlock()

                        protocol.Neighbor.AcceptedConn = nil
                    }))
                }
                protocol.Neighbor.acceptedConnMutex.Unlock()
            }
        }))

        go protocol.Init()
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
