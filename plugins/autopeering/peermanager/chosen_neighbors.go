package peermanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"

var CHOSEN_NEIGHBORS = &PeerList{make(map[string]*peer.Peer)}
