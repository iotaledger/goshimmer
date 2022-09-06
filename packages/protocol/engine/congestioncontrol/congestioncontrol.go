package congestioncontrol

import (
	"time"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
)

type CongestionControl struct {
	Events *Events

	optsScheduler []options.Option[scheduler.Scheduler]

	*scheduler.Scheduler
}

func New(isBlockAccepted func(models.BlockID) bool, blockAcceptedEvent *event.Linkable[*acceptance.Block, acceptance.Events, *acceptance.Events], tangle *tangle.Tangle, accessManaMapRetrieverFunc func() map[identity.ID]float64, totalAccessManaRetrieveFunc func() float64, rate time.Duration, opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(new(CongestionControl), opts, func(c *CongestionControl) {
		c.Scheduler = scheduler.New(isBlockAccepted, blockAcceptedEvent, tangle, accessManaMapRetrieverFunc, totalAccessManaRetrieveFunc, rate, c.optsScheduler...)

		c.Events = NewEvents()
		c.Events.Scheduler = c.Scheduler.Events
	})
}
