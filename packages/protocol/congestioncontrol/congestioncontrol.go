package congestioncontrol

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/slot"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/options"
)

type CongestionControl struct {
	Events *Events

	slotTimeProvider *slot.TimeProvider

	scheduler      *scheduler.Scheduler
	schedulerMutex sync.RWMutex

	optsSchedulerOptions []options.Option[scheduler.Scheduler]
}

func New(slotTimeProvider *slot.TimeProvider, opts ...options.Option[CongestionControl]) *CongestionControl {
	return options.Apply(&CongestionControl{
		Events:           NewEvents(),
		slotTimeProvider: slotTimeProvider,
	}, opts)
}

func (c *CongestionControl) Shutdown() {
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
		c.slotTimeProvider,
		engine.Consensus.BlockGadget.IsBlockAccepted,
		engine.ThroughputQuota.BalanceByIDs,
		engine.ThroughputQuota.TotalBalance,
		c.optsSchedulerOptions...,
	)
	c.Events.Scheduler.LinkTo(c.scheduler.Events)

	wp := engine.Workers.CreatePool("Scheduler", 2)
	engine.SubscribeStopped(
		engine.Tangle.Events.Booker.VirtualVoting.BlockTracked.Hook(c.scheduler.AddBlock, event.WithWorkerPool(wp)).Unhook,
		// event.AttachWithWorkerPool(engine.Tangle.Events.VirtualVoting.BlockTracked, func(block *virtualvoting.Block) {
		//	registerBlock, err := c.scheduler.GetOrRegisterBlock(block)
		//	if err != nil {
		//		panic(err)
		//	}
		//	c.Events.Scheduler.BlockScheduled.Trigger(registerBlock)
		// }, wp)
		engine.Tangle.Events.BlockDAG.BlockOrphaned.Hook(c.scheduler.HandleOrphanedBlock, event.WithWorkerPool(wp)).Unhook,
		engine.Consensus.Events.BlockGadget.BlockAccepted.Hook(c.scheduler.HandleAcceptedBlock, event.WithWorkerPool(wp)).Unhook,
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
