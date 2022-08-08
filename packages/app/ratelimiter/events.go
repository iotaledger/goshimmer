package ratelimiter

import (
	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/generics/event"
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
