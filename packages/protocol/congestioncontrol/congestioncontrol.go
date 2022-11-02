package congestioncontrol

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type CongestionControl struct {
	Events *Events

	scheduler      *scheduler.Scheduler
	schedulerMutex sync.RWMutex

	optsSchedulerOptions []options.Option[scheduler.Scheduler]
}

func New(opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	return options.Apply(&CongestionControl{
		Events: NewEvents(),
	}, opts)
}

func (c *CongestionControl) LinkTo(engine *engine.Engine) {
	c.schedulerMutex.Lock()
	defer c.schedulerMutex.Unlock()

	if c.scheduler != nil {
		c.scheduler.Shutdown()
	}

	c.scheduler = scheduler.New(
		engine.EvictionState,
		engine.Consensus.IsBlockAccepted,
		engine.ManaTracker.ManaMap,
		engine.ManaTracker.TotalMana,
		c.optsSchedulerOptions...,
	)

	engine.Tangle.Events.VirtualVoting.BlockTracked.Attach(event.NewClosure(c.scheduler.AddBlock))
	//engine.Tangle.Events.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
	//	registerBlock, err := c.scheduler.GetOrRegisterBlock(block)
	//	if err != nil {
	//		panic(err)
	//	}
	//	c.Events.Scheduler.BlockScheduled.Trigger(registerBlock)
	//}))
	engine.Tangle.Events.BlockDAG.BlockOrphaned.Attach(event.NewClosure(c.scheduler.HandleOrphanedBlock))
	engine.Consensus.Events.Acceptance.BlockAccepted.Attach(event.NewClosure(c.scheduler.HandleAcceptedBlock))

	c.Events.Scheduler.LinkTo(c.scheduler.Events)

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
