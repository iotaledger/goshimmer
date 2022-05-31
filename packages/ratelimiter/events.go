package ratelimiter

import (
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/event"
)

type Events struct {
	Hit *event.Event[*HitEvent]
}

func newEvents() (new *Events) {
	return &Events{
		Hit: event.New[*HitEvent](),
	}
}

type HitEvent struct {
	Peer      *peer.Peer
	RateLimit *RateLimit
}
