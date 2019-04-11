package peermanager

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager/types"
)

var KNOWN_PEERS = initKnownPeers()

func initKnownPeers() types.PeerRegister {
    knownPeers := make(types.PeerRegister)
    for _, entryNode := range ENTRY_NODES {
        knownPeers.AddOrUpdate(entryNode)
    }

    return knownPeers
}