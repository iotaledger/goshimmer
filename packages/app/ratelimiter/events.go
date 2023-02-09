package ratelimiter

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
)

type Events struct {
	Hit *event.Linkable[*HitEvent]
}

func newEvents() *Events {
	return &Events{
		Hit: event.NewLinkable[*HitEvent](),
	}
}

type HitEvent struct {
	Source    identity.ID
	RateLimit *RateLimit
}
