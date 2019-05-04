package acceptedneighbors

import (
    "github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
    "github.com/iotaledger/goshimmer/plugins/autopeering/saltmanager"
    "github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
    "hash/fnv"
)

var DISTANCE = func(anchor *peer.Peer) func(p *peer.Peer) uint64 {
    return func(p *peer.Peer) uint64 {
        saltedIdentifier := make([]byte, len(anchor.Identity.Identifier) + len(saltmanager.PRIVATE_SALT.Bytes))
        copy(saltedIdentifier[0:], anchor.Identity.Identifier)
        copy(saltedIdentifier[len(anchor.Identity.Identifier):], saltmanager.PRIVATE_SALT.Bytes)

        return hash(saltedIdentifier) ^ hash(p.Identity.Identifier)
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

