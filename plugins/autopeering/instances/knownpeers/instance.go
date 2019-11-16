package knownpeers

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/entrynodes"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerregister"
	"github.com/iotaledger/hive.go/node"
)

var INSTANCE *peerregister.PeerRegister

func Configure(plugin *node.Plugin) {
	INSTANCE = initKnownPeers()
}

func initKnownPeers() *peerregister.PeerRegister {
	knownPeers := peerregister.New()
	for _, entryNode := range entrynodes.INSTANCE.GetPeers() {
		knownPeers.AddOrUpdate(entryNode)
	}

	return knownPeers
}
