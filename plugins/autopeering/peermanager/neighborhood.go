package peermanager

import (
    "github.com/iotaledger/goshimmer/packages/timeutil"
    "github.com/iotaledger/goshimmer/plugins/autopeering/peermanager/types"
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/request"
    "time"
)

var NEIGHBORHOOD types.PeerRegister

var NEIGHBORHOOD_LIST types.PeerList

// Selects a fixed neighborhood from all known peers - this allows nodes to "stay in the same circles" that share their
// view on the ledger an is a preparation for economic clustering
var NEIGHBORHOOD_SELECTOR = func(this types.PeerRegister, req *request.Request) types.PeerRegister {
    filteredPeers := make(types.PeerRegister)
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
    if float64(len(NEIGHBORHOOD)) * 1.2 <= float64(len(KNOWN_PEERS)) || lastUpdate.Before(time.Now().Add(-300 * time.Second)) {
        NEIGHBORHOOD = KNOWN_PEERS.Filter(NEIGHBORHOOD_SELECTOR, request.OUTGOING_REQUEST)
        NEIGHBORHOOD_LIST = NEIGHBORHOOD.List()

        lastUpdate = time.Now()

        Events.UpdateNeighborhood.Trigger()
    }
}