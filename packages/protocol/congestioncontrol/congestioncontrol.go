package congestioncontrol

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

type CongestionControl struct {
	Events *Events

	workerPool     *workerpool.UnboundedWorkerPool
	scheduler      *scheduler.Scheduler
	schedulerMutex sync.RWMutex

	optsSchedulerOptions []options.Option[scheduler.Scheduler]
}

func New(opts ...options.Option[CongestionControl]) (congestionControl *CongestionControl) {
	congestionControl = options.Apply(&CongestionControl{
		Events:     NewEvents(),
		workerPool: workerpool.NewUnboundedWorkerPool(1),
	}, opts)

	congestionControl.workerPool.Start()

	return congestionControl
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
	engine.Tangle.Events.VirtualVoting.BlockTracked.AttachWithWorkerPool(event.NewClosure(c.scheduler.AddBlock), c.workerPool)
	// engine.Tangle.Events.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
	//	registerBlock, err := c.scheduler.GetOrRegisterBlock(block)
	//	if err != nil {
	//		panic(err)
	//	}
	//	c.Events.Scheduler.BlockScheduled.Trigger(registerBlock)
	// }))
	engine.Tangle.Events.BlockDAG.BlockOrphaned.AttachWithWorkerPool(event.NewClosure(c.scheduler.HandleOrphanedBlock), c.workerPool)
	engine.Consensus.Events.BlockGadget.BlockAccepted.AttachWithWorkerPool(event.NewClosure(c.scheduler.HandleAcceptedBlock), c.workerPool)

	c.Events.Scheduler.LinkTo(c.scheduler.Events)

	c.scheduler.Start()
}

func (c *CongestionControl) WorkerPool() *workerpool.UnboundedWorkerPool {
	c.schedulerMutex.RLock()
	defer c.schedulerMutex.RUnlock()

	return c.workerPool
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
