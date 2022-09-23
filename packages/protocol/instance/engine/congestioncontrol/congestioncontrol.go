package congestioncontrol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/mana/manamodels"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
)

type CongestionControl struct {
	Events *Events
	Gadget *acceptance.Gadget

	optsSchedulerOptions []options.Option[scheduler.Scheduler]
	*mana.Tracker
	*scheduler.Scheduler
}

func New(gadget *acceptance.Gadget, tangle *tangle.Tangle, opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(&CongestionControl{
		Events: NewEvents(),
		Gadget: gadget,
	}, opts, func(c *CongestionControl) {
		c.Tracker = mana.NewTracker(tangle.Ledger)
		c.Scheduler = scheduler.New(gadget.IsBlockAccepted, tangle, func() map[identity.ID]int64 {
			manaMap, _, err := c.Tracker.GetManaMap(manamodels.AccessMana)
			if err != nil {
				return make(map[identity.ID]int64)
			}
			return manaMap
		}, func() int64 {
			totalMana, _, err := c.Tracker.GetTotalMana(manamodels.AccessMana)
			if err != nil {
				return 0
			}
			return totalMana
		}, c.optsSchedulerOptions...)

		c.Events.Scheduler = c.Scheduler.Events
		c.Events.Tracker = c.Tracker.Events
	}, (*CongestionControl).setupEvents)
}

func (c *CongestionControl) setupEvents() {
	c.Gadget.Events.BlockAccepted.Attach(event.NewClosure(func(acceptedBlock *acceptance.Block) {
		c.HandleAcceptedBlock(acceptedBlock.Block)
	}))
}

func WithSchedulerOptions(opts ...options.Option[scheduler.Scheduler]) options.Option[CongestionControl] {
	return func(c *CongestionControl) {
		c.optsSchedulerOptions = opts
	}
}
