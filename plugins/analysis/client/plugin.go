package client

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/addnode"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/connectnodes"
    "github.com/iotaledger/goshimmer/plugins/analysis/types/ping"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/response"
    "github.com/iotaledger/goshimmer/plugins/gossip/neighbormanager"
    "net"
    "time"
)

func Run(plugin *node.Plugin) {
    daemon.BackgroundWorker(func() {
        shuttingDown := false
        for !shuttingDown {
            if conn, err := net.Dial("tcp", *SERVER_ADDRESS.Value); err != nil {
                plugin.LogDebug("Could not connect to reporting server: " + err.Error())
            } else {
                managedConn := network.NewManagedConnection(conn)
                eventDispatchers := getEventDispatchers(managedConn)

                reportCurrentStatus(eventDispatchers)
                setupHooks(managedConn, eventDispatchers)

                shuttingDown = keepConnectionAlive(managedConn)
            }
        }
    })
}

func getEventDispatchers(conn *network.ManagedConnection) *EventDispatchers {
    return &EventDispatchers{
        AddNode: func(nodeId []byte) {
            conn.Write((&addnode.Packet{ NodeId: nodeId }).Marshal())
        },
        ConnectNodes: func(sourceId []byte, targetId []byte) {
            conn.Write((&connectnodes.Packet{ SourceId: sourceId, TargetId: targetId }).Marshal())
        },
    }
}

func reportCurrentStatus(eventDispatchers *EventDispatchers) {
    eventDispatchers.AddNode(accountability.OWN_ID.Identifier)

    reportChosenNeighbors(eventDispatchers)
}

func setupHooks(conn *network.ManagedConnection, eventDispatchers *EventDispatchers) {
    // define hooks ////////////////////////////////////////////////////////////////////////////////////////////////////

    onDiscoverPeer := func(p *peer.Peer) {
        eventDispatchers.AddNode(p.Identity.Identifier)
    }

    onIncomingRequestAccepted := func(req *request.Request) {
        eventDispatchers.ConnectNodes(req.Issuer.Identity.Identifier, accountability.OWN_ID.Identifier)
    }

    onOutgoingRequestAccepted := func(res *response.Response) {
        eventDispatchers.ConnectNodes(accountability.OWN_ID.Identifier, res.Issuer.Identity.Identifier)
    }

    // setup hooks /////////////////////////////////////////////////////////////////////////////////////////////////////

    protocol.Events.DiscoverPeer.Attach(onDiscoverPeer)
    protocol.Events.IncomingRequestAccepted.Attach(onIncomingRequestAccepted)
    protocol.Events.OutgoingRequestAccepted.Attach(onOutgoingRequestAccepted)

    // clean up hooks on close /////////////////////////////////////////////////////////////////////////////////////////

    var onClose func()
    onClose = func() {
        protocol.Events.DiscoverPeer.Detach(onDiscoverPeer)
        protocol.Events.IncomingRequestAccepted.Detach(onIncomingRequestAccepted)
        protocol.Events.OutgoingRequestAccepted.Detach(onOutgoingRequestAccepted)

        conn.Events.Close.Detach(onClose)
    }
    conn.Events.Close.Attach(onClose)
}

func reportChosenNeighbors(dispatchers *EventDispatchers) {
    for _, chosenNeighbor := range neighbormanager.CHOSEN_NEIGHBORS {
        dispatchers.AddNode(chosenNeighbor.Identity.Identifier)
    }
    for _, chosenNeighbor := range neighbormanager.CHOSEN_NEIGHBORS {
        dispatchers.ConnectNodes(accountability.OWN_ID.Identifier, chosenNeighbor.Identity.Identifier)
    }
}

func keepConnectionAlive(conn *network.ManagedConnection) bool {
    go conn.Read(make([]byte, 1))

    ticker := time.NewTicker(1 * time.Second)
    for {
        select {
            case <- daemon.ShutdownSignal:
                return true

            case <- ticker.C:
                if _, err := conn.Write((&ping.Packet{}).Marshal()); err != nil {
                    return false
                }
        }
    }
}
