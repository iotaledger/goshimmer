package congestioncontrol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle"
)

type CongestionControl struct {
	Events *Events
	Gadget *acceptance.Gadget

	optsScheduler []options.Option[scheduler.Scheduler]

	*scheduler.Scheduler
}

func New(gadget *acceptance.Gadget, tangle *tangle.Tangle, accessManaMapRetrieverFunc func() map[identity.ID]float64, totalAccessManaRetrieveFunc func() float64, opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(new(CongestionControl), opts, func(c *CongestionControl) {
		c.Gadget = gadget
		c.Scheduler = scheduler.New(gadget.IsBlockAccepted, tangle, accessManaMapRetrieverFunc, totalAccessManaRetrieveFunc, c.optsScheduler...)

		c.Events = NewEvents()
		c.Events.Scheduler = c.Scheduler.Events
	}, (*CongestionControl).setupEvents)
}

func (c *CongestionControl) setupEvents() {
	c.Gadget.Events.BlockAccepted.Attach(event.NewClosure(func(acceptedBlock *acceptance.Block) {
		c.HandleAcceptedBlock(acceptedBlock.Block)
	}))
}
