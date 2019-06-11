package network

import "github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"

func GetKnownPeers() []string {
	peers := []string{}
	for _, peer := range knownpeers.INSTANCE.List() { // Does this read locks INSTANCE (*peerregister.PeerRegister)?
		peers = append(peers, peer.Identity.StringIdentifier)
	}
	return peers
}
