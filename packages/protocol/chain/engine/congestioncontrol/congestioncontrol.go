package congestioncontrol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/congestioncontrol/icca/scheduler"
	acceptance2 "github.com/iotaledger/goshimmer/packages/protocol/chain/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/chain/engine/tangle"
)

type CongestionControl struct {
	Events *Events
	Gadget *acceptance2.Gadget

	optsScheduler []options.Option[scheduler.Scheduler]

	*scheduler.Scheduler
}

func New(gadget *acceptance2.Gadget, tangle *tangle.Tangle, accessManaMapRetrieverFunc func() map[identity.ID]float64, totalAccessManaRetrieveFunc func() float64, opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(new(CongestionControl), opts, func(c *CongestionControl) {
		c.Gadget = gadget
		c.Scheduler = scheduler.New(gadget.IsBlockAccepted, tangle, accessManaMapRetrieverFunc, totalAccessManaRetrieveFunc, c.optsScheduler...)

		c.Events = NewEvents()
		c.Events.Scheduler = c.Scheduler.Events
	}, (*CongestionControl).setupEvents)
}

func (c *CongestionControl) setupEvents() {
	c.Gadget.Events.BlockAccepted.Attach(event.NewClosure(func(acceptedBlock *acceptance2.Block) {
		c.HandleAcceptedBlock(acceptedBlock.Block)
	}))
}
