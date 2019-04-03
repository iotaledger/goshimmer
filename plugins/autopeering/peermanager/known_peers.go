package peermanager

import "github.com/iotaledger/goshimmer/plugins/autopeering/protocol/peer"

var KNOWN_PEERS = &PeerList{make(map[string]*peer.Peer)}
