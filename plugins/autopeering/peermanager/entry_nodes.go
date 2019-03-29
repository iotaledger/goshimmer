package peermanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol"
    "net"
    "strconv"
    "strings"
)

func getEntryNodes() []*protocol.Peer {
    result := make([]*protocol.Peer, 0)

    for _, entryNodeDefinition := range strings.Fields(*parameters.ENTRY_NODES.Value) {
        if entryNodeDefinition == "" {
            continue
        }

        entryNode := &protocol.Peer{
            Identity: UNKNOWN_IDENTITY,
        }

        protocolBits := strings.Split(entryNodeDefinition, "://")
        if len(protocolBits) != 2 {
            panic("invalid entry in list of trusted entry nodes: " + entryNodeDefinition)
        }
        switch protocolBits[0] {
        case "tcp":
            entryNode.PeeringProtocolType = protocol.TCP_PROTOCOL
        case "udp":
            entryNode.PeeringProtocolType = protocol.UDP_PROTOCOL
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

var ENTRY_NODES = getEntryNodes()