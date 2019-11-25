package peerregister

import (
	"github.com/iotaledger/goshimmer/plugins/autopeering/types/peer"
	"github.com/iotaledger/hive.go/events"
)

type peerRegisterEvents struct {
	Add    *events.Event
	Update *events.Event
	Remove *events.Event
}

func peerCaller(handler interface{}, params ...interface{}) {
	handler.(func(*peer.Peer))(params[0].(*peer.Peer))
}
