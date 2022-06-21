package manaverse

import (
	"github.com/iotaledger/hive.go/generics/event"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type SchedulerEvents struct {
	BlockQueued    *event.Event[*tangle.Message]
	BlockScheduled *event.Event[*tangle.Message]
	BlockDropped   *event.Event[*tangle.Message]
}
