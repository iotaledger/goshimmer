package chosenneighbors

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "hash/fnv"
)

var DISTANCE = func(anchor *peer.Peer) func(p *peer.Peer) uint64 {
    return func(p *peer.Peer) uint64 {
        saltedIdentifier := make([]byte, len(anchor.Identity.Identifier) + len(anchor.Salt.Bytes))
        copy(saltedIdentifier[0:], anchor.Identity.Identifier)
        copy(saltedIdentifier[len(anchor.Identity.Identifier):], anchor.Salt.Bytes)

        return hash(anchor.Identity.Identifier) ^ hash(p.Identity.Identifier)
    }
}

var OWN_DISTANCE func(p *peer.Peer) uint64

func configureOwnDistance() {
    OWN_DISTANCE = DISTANCE(ownpeer.INSTANCE)
}

func hash(data []byte) uint64 {
    h := fnv.New64a()
    h.Write(data)

    return h.Sum64()
}
