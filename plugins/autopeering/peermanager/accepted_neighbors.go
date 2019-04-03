package peermanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"

var ACCEPTED_NEIGHBORS = &PeerList{make(map[string]*peer.Peer)}
