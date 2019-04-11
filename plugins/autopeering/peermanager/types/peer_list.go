package types

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"
    "sort"
)

type PeerList []*peer.Peer

func (this PeerList) Filter(predicate func(p *peer.Peer) bool) PeerList {
    peerList := make(PeerList, len(this))

    counter := 0
    for _, peer := range this {
        if predicate(peer) {
            peerList[counter] = peer
            counter++
        }
    }

    return peerList[:counter]
}

// Sorts the PeerRegister by their distance to an anchor.
func (this PeerList) Sort(distance func(p *peer.Peer) float64) PeerList {
    sort.Slice(this, func(i, j int) bool {
        return distance(this[i]) < distance(this[j])
    })

    return this
}


