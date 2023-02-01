package congestioncontrol

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type CongestionControl struct {
	Events *Events

	scheduler      *scheduler.Scheduler
	schedulerMutex sync.RWMutex

	optsSchedulerOptions []options.Option[scheduler.Scheduler]

	traits.Runnable
}

func New(opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	congestionControl = options.Apply(&CongestionControl{
		Events:   NewEvents(),
		Runnable: traits.NewRunnable(),
	}, opts)

	congestionControl.SubscribeStopped(
		congestionControl.shutdownScheduler,
	)

	return congestionControl
}

func (c *CongestionControl) shutdownScheduler() {
	c.schedulerMutex.RLock()
	defer c.schedulerMutex.RUnlock()

	c.scheduler.Shutdown()
}

func (c *CongestionControl) LinkTo(engine *engine.Engine) {
	c.schedulerMutex.Lock()
	defer c.schedulerMutex.Unlock()

	if c.scheduler != nil {
		c.scheduler.Shutdown()
	}

	c.scheduler = scheduler.New(
		engine.EvictionState,
		engine.Consensus.BlockGadget.IsBlockAccepted,
		engine.ThroughputQuota.BalanceByIDs,
		engine.ThroughputQuota.TotalBalance,
		c.optsSchedulerOptions...,
	)
	c.Events.Scheduler.LinkTo(c.scheduler.Events)

	wp := c.NewWorkerPool("Scheduler")
	engine.SubscribeStopped(
		c.SubscribeStopped(
			event.AttachWithWorkerPool(engine.Tangle.Events.VirtualVoting.BlockTracked, c.scheduler.AddBlock, wp),
			// engine.Tangle.Events.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
			//	registerBlock, err := c.scheduler.GetOrRegisterBlock(block)
			//	if err != nil {
			//		panic(err)
			//	}
			//	c.Events.Scheduler.BlockScheduled.Trigger(registerBlock)
			// }))
			event.AttachWithWorkerPool(engine.Tangle.Events.BlockDAG.BlockOrphaned, c.scheduler.HandleOrphanedBlock, wp),
			event.AttachWithWorkerPool(engine.Consensus.Events.BlockGadget.BlockAccepted, c.scheduler.HandleAcceptedBlock, wp),
		),
	)

	c.scheduler.Start()
}

func (c *CongestionControl) Scheduler() *scheduler.Scheduler {
	c.schedulerMutex.RLock()
	defer c.schedulerMutex.RUnlock()

	return c.scheduler
}

func (c *CongestionControl) Block(id models.BlockID) (block *scheduler.Block, exists bool) {
	c.schedulerMutex.RLock()
	defer c.schedulerMutex.RUnlock()

	return c.scheduler.Block(id)
}

func WithSchedulerOptions(opts ...options.Option[scheduler.Scheduler]) options.Option[CongestionControl] {
	return func(c *CongestionControl) {
		c.optsSchedulerOptions = opts
	}
}
