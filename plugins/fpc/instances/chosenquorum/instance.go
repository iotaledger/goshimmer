package chosenquorum

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/daemon"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/timeutil"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/knownpeers"
	"github.com/iotaledger/goshimmer/plugins/autopeering/instances/outgoingrequest"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerlist"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peerregister"
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/request"
	"github.com/iotaledger/goshimmer/plugins/fpc/parameters"

	"math"
	"math/rand"
	"time"
)

var INSTANCE *peerregister.PeerRegister

var LIST_INSTANCE peerlist.PeerList

var lastUpdate = time.Now()

// Selects a quorom from all known peers by random
var QUORUM_SELECTOR = func(this *peerregister.PeerRegister, req *request.Request) *peerregister.PeerRegister {
	rand.Seed(time.Now().UTC().UnixNano()) // ??? this seed needs to be increased by something that is private to the node
	selectedPeers := peerregister.New()
	fmt.Println("------- Peers ---------")
	fmt.Println("len(this.Peers)", len(this.Peers))

	keys := make([]string, len(this.Peers))
	i := 0
	for id, peer := range this.Peers {
		keys[i] = id
		fmt.Print(peer.PeeringPort, " ")
		i++
	}
	fmt.Println()
	fmt.Println(" - - selecting quorum - - ")
	for i = 0; i < *parameters.QUORUMSIZE.Value; i++ {
		randKey := keys[rand.Intn(int(math.Min(float64(*parameters.QUORUMSIZE.Value), float64(len(this.Peers)))))]
		selectedPeers.Peers[randKey] = this.Peers[randKey]
		fmt.Print("i", i, " ", this.Peers[randKey].PeeringPort, " , ")
	}
	fmt.Println()
	return selectedPeers
}

func Configure(plugin *node.Plugin) {
	updateQuorum()
}

func Run(plugin *node.Plugin) {
	daemon.BackgroundWorker(func() {
		timeutil.Ticker(updateQuorum, 30*time.Second) // ??? the time needs to be replaced by parameter ROUND_TIME
	})
}

func updateQuorum() {
	// select new quorum
	INSTANCE = knownpeers.INSTANCE.Filter(QUORUM_SELECTOR, outgoingrequest.INSTANCE) // do we need the last ???
	LIST_INSTANCE = INSTANCE.List()                                                  //??? what does this do?
	lastUpdate = time.Now()
	Events.Update.Trigger()

	fmt.Println("- - - - Quorum - - - - ")
	for _, quorumMember := range INSTANCE.Peers {
		fmt.Print(quorumMember.PeeringPort, " ")
	}
	fmt.Println()

}
