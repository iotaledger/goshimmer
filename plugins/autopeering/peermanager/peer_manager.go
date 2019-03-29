package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/accountability"
    "github.com/iotaledger/goshimmer/packages/daemon"
    "github.com/iotaledger/goshimmer/packages/network"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/salt"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/server"
    "math"
    "net"
    "strconv"
    "time"
)

var PEERING_REQUEST *protocol.PeeringRequest

var KNOWN_PEERS = &PeerList{make(map[string]*protocol.Peer)}

var CHOSEN_NEIGHBORS = &PeerList{make(map[string]*protocol.Peer)}

var ACCEPTED_NEIGHBORS = &PeerList{make(map[string]*protocol.Peer)}

func configurePeeringRequest() {
    PEERING_REQUEST = &protocol.PeeringRequest{
        Issuer: &protocol.Peer{
            Identity:            accountability.OWN_ID,
            PeeringProtocolType: protocol.TCP_PROTOCOL,
            PeeringPort:         uint16(*parameters.UDP_PORT.Value),
            GossipProtocolType:  protocol.TCP_PROTOCOL,
            GossipPort:          uint16(*parameters.UDP_PORT.Value),
            Address:             net.IPv4(0, 0, 0, 0),
        },
        Salt: saltmanager.PUBLIC_SALT,
    }
    PEERING_REQUEST.Sign()

    saltmanager.Events.UpdatePublicSalt.Attach(func(salt *salt.Salt) {
        PEERING_REQUEST.Sign()
    })
}

func Configure(plugin *node.Plugin) {
    configurePeeringRequest()

    server.Events.ReceivePeeringRequest.Attach(func(ip net.IP, peeringRequest *protocol.PeeringRequest) {
        peer := peeringRequest.Issuer
        peer.Address = ip

        KNOWN_PEERS.Update(peer)

        plugin.LogInfo("received peering request from " + peeringRequest.Issuer.Identity.StringIdentifier)
    })
    server.Events.ReceiveTCPPeeringRequest.Attach(func(conn network.Connection, request *protocol.PeeringRequest) {
        peer := request.Issuer
        peer.Address = conn.GetConnection().RemoteAddr().(*net.TCPAddr).IP

        KNOWN_PEERS.Update(peer)

        plugin.LogInfo("received peering request from " + request.Issuer.Identity.StringIdentifier)

        sendPeeringResponse(conn.GetConnection())
    })
    server.Events.Error.Attach(func(ip net.IP, err error) {
        plugin.LogFailure("invalid peering request from " + ip.String())
    })
}

func Run(plugin *node.Plugin) {
    daemon.BackgroundWorker(func() {
        chooseNeighbors(plugin)

        ticker := time.NewTicker(FIND_NEIGHBOR_INTERVAL)
        for {
            select {
            case <- daemon.ShutdownSignal:
                return
            case <- ticker.C:
                chooseNeighbors(plugin)
            }
        }
    })
}

func Shutdown(plugin *node.Plugin) {}

func generateProposedNodeCandidates() []*protocol.Peer {
    peers := make([]*protocol.Peer, 0)
    for _, peer := range KNOWN_PEERS.Peers {
        peers = append(peers, peer)
    }

    return peers
}

func rejectPeeringRequest(conn net.Conn) {
    conn.Write((&protocol.PeeringResponse{
        Type:   protocol.PEERING_RESPONSE_REJECT,
        Issuer: PEERING_REQUEST.Issuer,
        Peers:  generateProposedNodeCandidates(),
    }).Sign().Marshal())
    conn.Close()
}

func acceptPeeringRequest(conn net.Conn) {
    conn.Write((&protocol.PeeringResponse{
        Type:   protocol.PEERING_RESPONSE_ACCEPT,
        Issuer: PEERING_REQUEST.Issuer,
        Peers:  generateProposedNodeCandidates(),
    }).Sign().Marshal())
    conn.Close()
}

func sendPeeringResponse(conn net.Conn) {
    if len(ACCEPTED_NEIGHBORS.Peers) < protocol.NEIGHBOR_COUNT / 2 {
        acceptPeeringRequest(conn)
    } else {
        rejectPeeringRequest(conn)
    }
}

func sendPeeringRequest(plugin *node.Plugin, peer *protocol.Peer) {
    var protocolString string
    switch peer.PeeringProtocolType {
    case protocol.TCP_PROTOCOL:
        protocolString = "tcp"
    case protocol.UDP_PROTOCOL:
        protocolString = "udp"
    default:
        panic("invalid protocol in known peers")
    }

    conn, err := net.Dial(protocolString, peer.Address.String() + ":" + strconv.Itoa(int(peer.PeeringPort)))
    if err != nil {
        plugin.LogFailure(err.Error())
    } else {
        conn := network.NewPeer(protocolString, conn)

        conn.Write(PEERING_REQUEST.Marshal())

        buffer := make([]byte, protocol.PEERING_RESPONSE_MARSHALLED_TOTAL_SIZE)
        offset := 0
        conn.OnReceiveData(func(data []byte) {
            remainingCapacity := int(math.Min(float64(protocol.PEERING_RESPONSE_MARSHALLED_TOTAL_SIZE - offset), float64(len(data))))

            copy(buffer[offset:], data[:remainingCapacity])
            offset += len(data)

            if offset >= protocol.PEERING_RESPONSE_MARSHALLED_TOTAL_SIZE {
                peeringResponse, err := protocol.UnmarshalPeeringResponse(buffer)
                if err != nil {
                    plugin.LogFailure("invalid peering response from " + conn.GetConnection().RemoteAddr().String())
                } else {
                    processPeeringResponse(plugin, peeringResponse)
                }

                conn.GetConnection().Close()
            }
        })

        go conn.HandleConnection()
    }
}

func processPeeringResponse(plugin *node.Plugin, response *protocol.PeeringResponse) {
    KNOWN_PEERS.Update(response.Issuer)
    for _, peer := range response.Peers {
        KNOWN_PEERS.Update(peer)
    }

    switch response.Type {
    case protocol.PEERING_RESPONSE_ACCEPT:
        CHOSEN_NEIGHBORS.Update(response.Issuer)
    case protocol.PEERING_RESPONSE_REJECT:
    default:
        plugin.LogInfo("invalid response type in peering response of " + response.Issuer.Address.String() + ":" + strconv.Itoa(int(response.Issuer.PeeringPort)))
    }
}

func getChosenNeighborCandidates() []*protocol.Peer {
    result := make([]*protocol.Peer, 0)

    for _, peer := range KNOWN_PEERS.Peers {
        result = append(result, peer)
    }

    for _, peer := range ENTRY_NODES {
        result = append(result, peer)
    }

    return result
}

func chooseNeighbors(plugin *node.Plugin) {
    for _, peer := range getChosenNeighborCandidates() {
        sendPeeringRequest(plugin, peer)
    }
}
