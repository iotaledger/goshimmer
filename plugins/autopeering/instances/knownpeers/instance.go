package knownpeers

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/entrynodes"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerregister"
)

var INSTANCE = initKnownPeers()

func initKnownPeers() peerregister.PeerRegister {
    knownPeers := make(peerregister.PeerRegister)
    for _, entryNode := range entrynodes.INSTANCE {
        knownPeers.AddOrUpdate(entryNode)
    }

    return knownPeers
}
