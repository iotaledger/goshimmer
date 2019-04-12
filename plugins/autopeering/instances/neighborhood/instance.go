package neighborhood

import (
    "github.com/iotaledger/goshimmer/packages/timeutil"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peerregister"
    "time"
)

var INSTANCE peerregister.PeerRegister

var LIST_INSTANCE peerlist.PeerList

// Selects a fixed neighborhood from all known peers - this allows nodes to "stay in the same circles" that share their
// view on the ledger an is a preparation for economic clustering
var NEIGHBORHOOD_SELECTOR = func(this peerregister.PeerRegister, req *request.Request) peerregister.PeerRegister {
    filteredPeers := make(peerregister.PeerRegister)
    for id, peer := range this {
        filteredPeers[id] = peer
    }

    return filteredPeers
}

var lastUpdate = time.Now()

func init() {
    updateNeighborHood()

    go timeutil.Ticker(updateNeighborHood, 1 * time.Second)
}

func updateNeighborHood() {
    if float64(len(INSTANCE)) * 1.2 <= float64(len(knownpeers.INSTANCE)) || lastUpdate.Before(time.Now().Add(-300 * time.Second)) {
        INSTANCE = knownpeers.INSTANCE.Filter(NEIGHBORHOOD_SELECTOR, outgoingrequest.INSTANCE)
        LIST_INSTANCE = INSTANCE.List()

        lastUpdate = time.Now()

        Events.Update.Trigger()
    }
}
