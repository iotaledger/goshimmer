package ratelimiter

import (
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/runtime/event"
)

type Events struct {
	Hit *event.Event1[*HitEvent]
}

func newEvents() *Events {
	return &Events{
		Hit: event.New1[*HitEvent](),
	}
}

type HitEvent struct {
	Source    identity.ID
	RateLimit *RateLimit
}
