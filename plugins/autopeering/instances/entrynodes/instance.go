package entrynodes

import (
    "encoding/hex"
    "github.com/iotaledger/goshimmer/packages/identity"
    "github.com/iotaledger/goshimmer/packages/node"
    "github.com/iotaledger/goshimmer/plugins/autopeering/parameters"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "net"
    "strconv"
    "strings"
)

var INSTANCE peerlist.PeerList

func Configure(node *node.Plugin) {
    INSTANCE = parseEntryNodes()
}

func parseEntryNodes() peerlist.PeerList {
    result := make(peerlist.PeerList, 0)

    for _, entryNodeDefinition := range strings.Fields(*parameters.ENTRY_NODES.Value) {
        if entryNodeDefinition == "" {
            continue
        }

        entryNode := &peer.Peer{
            Identity: nil,
        }

        identityBits := strings.Split(entryNodeDefinition, "@")
        if len(identityBits) != 2 {
            panic("error while parsing identity of entry node: " + entryNodeDefinition)
        }
        if decodedIdentifier, err := hex.DecodeString(identityBits[0]); err != nil {
            panic("error while parsing identity of entry node: " + entryNodeDefinition)
        } else {
            entryNode.Identity = &identity.Identity{
                Identifier: decodedIdentifier,
                StringIdentifier: identityBits[0],
            }
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
