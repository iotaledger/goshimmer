package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    peermanagerTypes "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/types"
    "net"
    "strconv"
    "strings"
)

var ENTRY_NODES = parseEntryNodes()

func parseEntryNodes() peermanagerTypes.PeerList {
    result := make(peermanagerTypes.PeerList, 0)

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
            entryNode.PeeringProtocolType = types.PROTOCOL_TYPE_TCP
        case "udp":
            entryNode.PeeringProtocolType = types.PROTOCOL_TYPE_UDP
        }

        identityBits := strings.Split(protocolBits[1], "@")
        if len(identityBits) != 2 {
            panic("error while parsing identity of entry node: " + entryNodeDefinition)
        }
        entryNode.Identity = &identity.Identity{
            StringIdentifier: identityBits[0],
        }

        addressBits := strings.Split(identityBits[1], ":")
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
