package peermanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "net"
    "strconv"
    "strings"
)

var ENTRY_NODES = parseEntryNodes()

func parseEntryNodes() []*peer.Peer {
    result := make([]*peer.Peer, 0)

    for _, entryNodeDefinition := range strings.Fields(*parameters.ENTRY_NODES.Value) {
        if entryNodeDefinition == "" {
            continue
        }

        entryNode := &peer.Peer{
            Identity: nil,
        }

        protocolBits := strings.Split(entryNodeDefinition, "://")
        if len(protocolBits) != 2 {
            panic("invalid entry in list of trusted entry nodes: " + entryNodeDefinition)
        }
        switch protocolBits[0] {
        case "tcp":
            entryNode.PeeringProtocolType = protocol.PROTOCOL_TYPE_TCP
        case "udp":
            entryNode.PeeringProtocolType = protocol.PROTOCOL_TYPE_UDP
        }

        addressBits := strings.Split(protocolBits[1], ":")
        switch len(addressBits) {
        case 2:
            host := addressBits[0]
            port, err := strconv.Atoi(addressBits[1])
            if err != nil {
                panic("error while parsing port of entry in list of entry nodes")
            }

            ip := net.ParseIP(host)
            if ip == nil {
                panic("error while parsing ip of entry in list of entry nodes")
            }

            entryNode.Address = ip
            entryNode.PeeringPort = uint16(port)
        case 6:
            host := strings.Join(addressBits[:5], ":")
            port, err := strconv.Atoi(addressBits[5])
            if err != nil {
                panic("error while parsing port of entry in list of entry nodes")
            }

            ip := net.ParseIP(host)
            if ip == nil {
                panic("error while parsing ip of entry in list of entry nodes")
            }

            entryNode.Address = ip
            entryNode.PeeringPort = uint16(port)
        default:
            panic("invalid entry in list of trusted entry nodes: " + entryNodeDefinition)
        }

        result = append(result, entryNode)
    }

    return result
}
