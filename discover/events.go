package discover

import (
	"github.com/iotaledger/autopeering-sim/peer"
	"github.com/iotaledger/hive.go/events"
)

// Events contains all the events that are triggered during the peer discovery.
var Events = struct {
	// A PeerDiscovered event is triggered, when a new peer has been discovered and verified.
	PeerDiscovered *events.Event
}{
	PeerDiscovered: events.NewEvent(peerDiscovered),
}

// DiscoveredEvent bundles the information of the discovered peer.
type DiscoveredEvent struct {
	Peer *peer.Peer // discovered peer
	//Services peer.ServiceMap // services supported by that peer
}

func peerDiscovered(handler interface{}, params ...interface{}) {
	handler.(func(*DiscoveredEvent))(params[0].(*DiscoveredEvent))
}
