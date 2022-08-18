package eviction

import (
	"github.com/iotaledger/hive.go/core/generics/event"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type Events struct {
	EpochEvicted *event.Event[epoch.Index]
}

func newEvents() (newEvents *Events) {
	return &Events{
		EpochEvicted: event.New[epoch.Index](),
	}
}
