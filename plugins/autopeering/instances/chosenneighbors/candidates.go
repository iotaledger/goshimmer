package chosenneighbors

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/neighborhood"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/ownpeer"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
)

var CANDIDATES *peerlist.PeerList

func configureCandidates() {
	CANDIDATES = peerlist.NewPeerList()
	updateNeighborCandidates()

	neighborhood.Events.Update.Attach(updateNeighborCandidates)
}

func updateNeighborCandidates() {
	CANDIDATES.Update(neighborhood.LIST_INSTANCE.Sort(DISTANCE(ownpeer.INSTANCE)).GetPeers())
}
