package ratelimiter

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
)

type Events struct {
	Hit *event.Event[*HitEvent]
}

func newEvents() *Events {
	return &Events{
		Hit: event.New[*HitEvent](),
	}
}

type HitEvent struct {
	Source    identity.ID
	RateLimit *RateLimit
}
