package neighborhood

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerregister"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/node"
)

var INSTANCE *peerregister.PeerRegister

var LIST_INSTANCE *peerlist.PeerList

// Selects a fixed neighborhood from all known peers - this allows nodes to "stay in the same circles" that share their
// view on the ledger an is a preparation for economic clustering
var NEIGHBORHOOD_SELECTOR = func(this *peerregister.PeerRegister, req *request.Request) *peerregister.PeerRegister {
	filteredPeers := peerregister.New()
	for id, peer := range this.Peers.GetMap() {
		filteredPeers.Peers.Store(id, peer)
	}

	return filteredPeers
}

var lastUpdate = time.Now()

func Configure(plugin *node.Plugin) {
	LIST_INSTANCE = peerlist.NewPeerList()
	updateNeighborHood()
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker("Neighborhood Updater", func() {
		timeutil.Ticker(updateNeighborHood, 1*time.Second)
	})
}

func updateNeighborHood() {
	if INSTANCE == nil || float64(INSTANCE.Peers.Len())*1.2 <= float64(knownpeers.INSTANCE.Peers.Len()) || lastUpdate.Before(time.Now().Add(-300*time.Second)) {
		INSTANCE = knownpeers.INSTANCE.Filter(NEIGHBORHOOD_SELECTOR, outgoingrequest.INSTANCE)

		LIST_INSTANCE.Update(INSTANCE.List())

		lastUpdate = time.Now()

		Events.Update.Trigger()
	}
}
