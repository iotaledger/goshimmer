package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/response"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/tcp"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server/udp"
    "net"
    "strconv"
    "time"
)

func Configure(plugin *node.Plugin) {
    configurePeeringRequest()

    // setup processing of peering requests
    udp.Events.ReceiveRequest.Attach(func(peeringRequest *request.Request) {
        processPeeringRequest(plugin, peeringRequest, nil)
    })
    tcp.Events.ReceiveRequest.Attach(func(conn *network.ManagedConnection, peeringRequest *request.Request) {
        processPeeringRequest(plugin, peeringRequest, conn)
    })

    // setup processing of peering responses
    udp.Events.ReceiveResponse.Attach(func(peeringResponse *response.Response) {
        processPeeringResponse(plugin, peeringResponse, nil)
    })
    tcp.Events.ReceiveResponse.Attach(func(conn *network.ManagedConnection, peeringResponse *response.Response) {
        processPeeringResponse(plugin, peeringResponse, conn)
    })

    udp.Events.Error.Attach(func(ip net.IP, err error) {
        plugin.LogDebug("error when communicating with " + ip.String() + ": " + err.Error())
    })
    tcp.Events.Error.Attach(func(ip net.IP, err error) {
        plugin.LogDebug("invalid peering request from " + ip.String() + ": " + err.Error())
    })
}

func Run(plugin *node.Plugin) {
    // setup background worker that contacts "chosen neighbors"
    daemon.BackgroundWorker(func() {
        plugin.LogInfo("Starting Peer Manager ...")
        plugin.LogSuccess("Starting Peer Manager ... done")

        sendPeeringRequests(plugin)

        ticker := time.NewTicker(FIND_NEIGHBOR_INTERVAL)
        for {
            select {
            case <-daemon.ShutdownSignal:
                return
            case <-ticker.C:
                sendPeeringRequests(plugin)
            }
        }
    })
}

func Shutdown(plugin *node.Plugin) {
    plugin.LogInfo("Stopping Peer Manager ...")
    plugin.LogSuccess("Stopping Peer Manager ... done")
}

func processPeeringRequest(plugin *node.Plugin, peeringRequest *request.Request, conn net.Conn) {
    if KNOWN_PEERS.Update(peeringRequest.Issuer) {
        plugin.LogInfo("new peer detected: " + peeringRequest.Issuer.Address.String() + " / " + peeringRequest.Issuer.Identity.StringIdentifier)
    }

    if conn == nil {
        plugin.LogDebug("received UDP peering request from " + peeringRequest.Issuer.Identity.StringIdentifier)

        var protocolString string
        switch peeringRequest.Issuer.PeeringProtocolType {
        case protocol.PROTOCOL_TYPE_TCP:
            protocolString = "tcp"
        case protocol.PROTOCOL_TYPE_UDP:
            protocolString = "udp"
        default:
            plugin.LogFailure("unsupported peering protocol in request from " + peeringRequest.Issuer.Address.String())

            return
        }

        var err error
        conn, err = net.Dial(protocolString, peeringRequest.Issuer.Address.String() + ":" + strconv.Itoa(int(peeringRequest.Issuer.PeeringPort)))
        if err != nil {
            plugin.LogDebug("error when connecting to " + peeringRequest.Issuer.Address.String() + " during peering process: " + err.Error())

            return
        }
    } else {
        plugin.LogDebug("received TCP peering request from " + peeringRequest.Issuer.Identity.StringIdentifier)
    }

    sendFittingPeeringResponse(conn)
}

func processPeeringResponse(plugin *node.Plugin, peeringResponse *response.Response, conn *network.ManagedConnection) {
    if KNOWN_PEERS.Update(peeringResponse.Issuer) {
        plugin.LogInfo("new peer detected: " + peeringResponse.Issuer.Address.String() + " / " + peeringResponse.Issuer.Identity.StringIdentifier)
    }
    for _, peer := range peeringResponse.Peers {
        if KNOWN_PEERS.Update(peer) {
            plugin.LogInfo("new peer detected: " + peer.Address.String() + " / " + peer.Identity.StringIdentifier)
        }
    }

    if conn == nil {
        plugin.LogDebug("received UDP peering response from " + peeringResponse.Issuer.Identity.StringIdentifier)
    } else {
        plugin.LogDebug("received TCP peering response from " + peeringResponse.Issuer.Identity.StringIdentifier)

        conn.Close()
    }

    switch peeringResponse.Type {
    case response.TYPE_ACCEPT:
        CHOSEN_NEIGHBORS.Update(peeringResponse.Issuer)
    case response.TYPE_REJECT:
    default:
        plugin.LogDebug("invalid response type in peering response of " + peeringResponse.Issuer.Address.String() + ":" + strconv.Itoa(int(peeringResponse.Issuer.PeeringPort)))
    }
}

func sendFittingPeeringResponse(conn net.Conn) {
    var peeringResponse *response.Response
    if len(ACCEPTED_NEIGHBORS.Peers) < protocol.NEIGHBOR_COUNT/2 {
        peeringResponse = generateAcceptResponse()
    } else {
        peeringResponse = generateRejectResponse()
    }

    peeringResponse.Sign()

    conn.Write(peeringResponse.Marshal())
    conn.Close()
}

func generateAcceptResponse() *response.Response {
    peeringResponse := &response.Response{
        Type:   response.TYPE_ACCEPT,
        Issuer: PEERING_REQUEST.Issuer,
        Peers:  generateProposedNodeCandidates(),
    }

    return peeringResponse
}

func generateRejectResponse() *response.Response {
    peeringResponse := &response.Response{
        Type:   response.TYPE_REJECT,
        Issuer: PEERING_REQUEST.Issuer,
        Peers:  generateProposedNodeCandidates(),
    }

    return peeringResponse
}


func generateProposedNodeCandidates() []*peer.Peer {
    peers := make([]*peer.Peer, 0)
    for _, peer := range KNOWN_PEERS.Peers {
        peers = append(peers, peer)
    }

    return peers
}

//region PEERING REQUEST RELATED METHODS ///////////////////////////////////////////////////////////////////////////////

func sendPeeringRequests(plugin *node.Plugin) {
    for _, peer := range getChosenNeighborCandidates() {
        sendPeeringRequest(plugin, peer)
    }
}

func sendPeeringRequest(plugin *node.Plugin, peer *peer.Peer) {
    switch peer.PeeringProtocolType {
    case protocol.PROTOCOL_TYPE_TCP:
        sendTCPPeeringRequest(plugin, peer)

    case protocol.PROTOCOL_TYPE_UDP:
        sendUDPPeeringRequest(plugin, peer)

    default:
        panic("invalid protocol in known peers")
    }
}

func sendTCPPeeringRequest(plugin *node.Plugin, peer *peer.Peer) {
    go func() {
        tcpConnection, err := net.Dial("tcp", peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)))
        if err != nil {
            plugin.LogDebug("error while trying to send TCP peering request to " + peer.String() + ": " + err.Error())
        } else {
            mConn := network.NewManagedConnection(tcpConnection)

            plugin.LogDebug("sending TCP peering request to " + peer.String())

            if _, err := mConn.Write(PEERING_REQUEST.Marshal()); err != nil {
                plugin.LogDebug("error while trying to send TCP peering request to " + peer.String() + ": " + err.Error())

                return
            }

            tcp.HandleConnection(mConn)
        }
    }()
}

func sendUDPPeeringRequest(plugin *node.Plugin, peer *peer.Peer) {
    go func() {
        udpConnection, err := net.Dial("udp", peer.Address.String()+":"+strconv.Itoa(int(peer.PeeringPort)))
        if err != nil {
            plugin.LogDebug("error while trying to send peering request to " + peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)) + " / " + peer.Identity.StringIdentifier + ": " + err.Error())
        } else {
            mConn := network.NewManagedConnection(udpConnection)

            if _, err := mConn.Write(PEERING_REQUEST.Marshal()); err != nil {
                plugin.LogDebug("error while trying to send peering request to " + peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)) + " / " + peer.Identity.StringIdentifier + ": " + err.Error())

                return
            }

            // setup listener for incoming responses
        }
    }()
}

func getChosenNeighborCandidates() []*peer.Peer {
    result := make([]*peer.Peer, 0)

    for _, peer := range KNOWN_PEERS.Peers {
        result = append(result, peer)
    }

    for _, peer := range ENTRY_NODES {
        result = append(result, peer)
    }

    return result
}

//endregion ////////////////////////////////////////////////////////////////////////////////////////////////////////////