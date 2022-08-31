package scheduler

import (
	"math/big"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/typeutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/acceptancegadget"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/schedulerutils"
	"github.com/iotaledger/goshimmer/packages/node/clock"
)

const (
	// MinMana is the minimum amount of Mana needed to issue blocks.
	// MaxBlockSize / MinMana is also the upper bound of iterations inside one schedule call, as such it should not be too small.
	MinMana float64 = 1.0
)

// MaxDeficit is the maximum cap for accumulated deficit, i.e. max bytes that can be scheduled without waiting.
// It must be >= MaxBlockSize.
var MaxDeficit = new(big.Rat).SetInt64(int64(MaxBlockSize))

// ErrNotRunning is returned when a block is submitted when the scheduler has been stopped.
var ErrNotRunning = errors.New("scheduler stopped")

// Scheduler is a Tangle component that takes care of scheduling the blocks that shall be booked.
type Scheduler struct {
	Events           *Events
	Tangle           *tangle.Tangle
	AcceptanceGadget *acceptancegadget.AcceptanceGadget
	EvictionManager  *eviction.LockableManager[models.BlockID]

	blocks        *memstorage.EpochStorage[models.BlockID, *Block]
	ticker        *time.Ticker
	bufferMutex   sync.RWMutex
	buffer        *schedulerutils.BufferQueue
	deficitsMutex sync.RWMutex
	deficits      map[identity.ID]*big.Rat

	optsRate                            *atomic.Duration
	optsMaxBufferSize                   int
	optsTotalAccessManaRetrieveFunc     func() float64
	optsAccessManaMapRetrieverFunc      func() map[identity.ID]float64
	optsConfirmedBlockScheduleThreshold time.Duration

	started        typeutils.AtomicBool
	stopped        typeutils.AtomicBool
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

// NewScheduler returns a new Scheduler.
func NewScheduler(acceptanceGadget *acceptancegadget.AcceptanceGadget, opts ...options.Option[Scheduler]) *Scheduler {
	return options.Apply(&Scheduler{
		Events:           newEvents(),
		Tangle:           acceptanceGadget.Tangle,
		AcceptanceGadget: acceptanceGadget,
		EvictionManager:  acceptanceGadget.EvictionManager.Lockable(),

		deficits: make(map[identity.ID]*big.Rat),

		optsMaxBufferSize:                   300,
		optsConfirmedBlockScheduleThreshold: 5 * time.Minute,

		shutdownSignal: make(chan struct{}),
	}, opts, func(s *Scheduler) {
		if s.optsAccessManaMapRetrieverFunc == nil || s.optsTotalAccessManaRetrieveFunc == nil {
			panic("scheduler: the option AccessManaMapRetrieverFunc and TotalAccessManaRetriever must be defined so that AccessMana can be determined in Scheduler")
		}

		if s.optsRate == nil {
			panic("scheduler: the option Rate needs to be defined")
		}

		// maximum access mana-scaled inbox length
		s.ticker = time.NewTicker(s.optsRate.Load())
		s.buffer = schedulerutils.NewBufferQueue(s.optsMaxBufferSize)

	}, (*Scheduler).setupEvents)

}

func (s *Scheduler) setupEvents() {
	// pass booked blocks to the scheduler
	s.Tangle.VirtualVoting.Events.BlockTracked.Attach(event.NewClosure(func(sourceBlock *virtualvoting.Block) {
		block, _ := s.getOrRegisterBlock(sourceBlock)

		if err := s.Submit(block); err != nil {
			if !errors.Is(err, schedulerutils.ErrInsufficientMana) {
				s.Events.Error.Trigger(errors.Errorf("failed to submit to scheduler: %w", err))
			}
		}
		s.tryReady(block)
	}))

	s.Events.BlockScheduled.Hook(event.NewClosure(func(block *Block) {
		s.updateChildren(block)
	}))

	onBlockAccepted := func(block *Block) {
		scheduled := block.Scheduled()
		if scheduled {
			return
		}
		if clock.Since(block.IssuingTime()) > s.optsConfirmedBlockScheduleThreshold {
			s.Unsubmit(block)
			block.SetSkipped()
			s.Events.BlockSkipped.Trigger(block)
		}
		s.updateChildren(block)
	}

	s.AcceptanceGadget.Events.BlockAccepted.Attach(event.NewClosure(func(sourceBlock *acceptancegadget.Block) {
		block, _ := s.getOrRegisterBlock(sourceBlock.Block)

		onBlockAccepted(block)
	}))
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	s.started.Set()
	// start the main loop
	go s.mainLoop()
}

// Running returns true if the scheduler has started.
func (s *Scheduler) Running() bool {
	return s.started.IsSet()
}

// Rate gets the rate of the scheduler.
func (s *Scheduler) Rate() time.Duration {
	return s.optsRate.Load()
}

// NodeQueueSize returns the size of the nodeIDs queue.
func (s *Scheduler) NodeQueueSize(nodeID identity.ID) int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	nodeQueue := s.buffer.NodeQueue(nodeID)
	if nodeQueue == nil {
		return 0
	}
	return nodeQueue.Size()
}

// NodeQueueSizes returns the size for each node queue.
func (s *Scheduler) NodeQueueSizes() map[identity.ID]int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	nodeQueueSizes := make(map[identity.ID]int)
	for _, nodeID := range s.buffer.NodeIDs() {
		size := s.buffer.NodeQueue(nodeID).Size()
		nodeQueueSizes[nodeID] = size
	}
	return nodeQueueSizes
}

// MaxBufferSize returns the max size of the buffer.
func (s *Scheduler) MaxBufferSize() int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.MaxSize()
}

// BufferSize returns the size of the buffer.
func (s *Scheduler) BufferSize() int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.Size()
}

// ReadyBlocksCount returns the size buffer.
func (s *Scheduler) ReadyBlocksCount() int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.ReadyBlocksCount()
}

// TotalBlocksCount returns the size buffer.
func (s *Scheduler) TotalBlocksCount() int {
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.TotalBlocksCount()
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		// lock the scheduler to make sure that any Submit() has been finished
		s.bufferMutex.Lock()
		defer s.bufferMutex.Unlock()
		s.stopped.Set()
		close(s.shutdownSignal)
	})
}

// Submit submits a block to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(blockID) has been called.
func (s *Scheduler) Submit(block *Block) (err error) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	return s.submit(block)
}

// Unsubmit removes a block from the submitted blocks.
// If that block is already marked as ready, Unsubmit has no effect.
func (s *Scheduler) Unsubmit(block *Block) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	s.unsubmit(block)
}

// Ready marks a previously submitted block as ready to be scheduled.
// If Ready is called without a previous Submit, it has no effect.
func (s *Scheduler) Ready(block *Block) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	s.ready(block)
}

// SubmitAndReady submits the block to the scheduler and marks it ready right away.
func (s *Scheduler) SubmitAndReady(block *Block) (err error) {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	if err = s.submit(block); err != nil {
		return err
	}

	s.ready(block)

	return nil
}

// isEligible returns true if the given blockID has either been scheduled or confirmed.
func (s *Scheduler) isEligible(block *Block) (eligible bool) {
	return block.Scheduled() || s.AcceptanceGadget.IsBlockAccepted(block.ID())
}

// isReady returns true if the given blockID's parents are eligible.
func (s *Scheduler) isReady(block *Block) (ready bool) {
	ready = true
	block.ForEachParent(func(parent models.Parent) {
		if parentBlock, parentExists := s.block(parent.ID); !parentExists || !s.isEligible(parentBlock) {
			ready = false
			return
		}
	})

	return
}

// tryReady tries to set the given block as ready.
func (s *Scheduler) tryReady(block *Block) {
	if s.isReady(block) {
		s.Ready(block)
	}
}

// updateChildren iterates over the direct children of the given blockID and
// tries to mark them as ready.
func (s *Scheduler) updateChildren(block *Block) {
	for _, childBlock := range block.Children() {
		if childBlockScheduler, childBlockExists := s.block(childBlock.ID()); childBlockExists {
			s.tryReady(childBlockScheduler)
		}
	}
}

func (s *Scheduler) submit(block *Block) error {
	if s.stopped.IsSet() {
		return ErrNotRunning
	}

	// when removing the zero mana node solution, check if nodes have MinMana here
	droppedBlockIDs, err := s.buffer.Submit(block, s.AccessManaCache().GetCachedMana)
	if err != nil {
		panic(errors.Errorf("failed to submit %s: %w", block.ID(), err))
	}
	for _, droppedBlockID := range droppedBlockIDs {
		discardedBlock, _ := s.block(droppedBlockID)
		s.Tangle.SetOrphaned(discardedBlock.Block.Block.Block, true)
		s.Events.BlockDiscarded.Trigger(discardedBlock)
	}
	return nil
}

func (s *Scheduler) unsubmit(block *Block) {
	s.buffer.Unsubmit(block)
}

func (s *Scheduler) ready(block *Block) {
	s.buffer.Ready(block)
}

func (s *Scheduler) Quanta(nodeID identity.ID) *big.Rat {
	return big.NewRat(s.GetManaFromCache(nodeID), int64(s.AccessManaCache().GetCachedTotalMana()))
}

// mainLoop periodically triggers the scheduling of ready blocks.
func (s *Scheduler) mainLoop() {
	defer s.ticker.Stop()

loop:
	for {
		select {
		// every rate time units
		case <-s.ticker.C:
			// TODO: pause the ticker, if there are no ready blocks
			if block := s.schedule(); block != nil {
				if block.SetScheduled() {
					s.Events.BlockScheduled.Trigger(block)
				}
			}

		// on close, exit the loop
		case <-s.shutdownSignal:
			break loop
		}
	}
}

// block retrieves the Block with given id from the mem-storage.
func (s *Scheduler) block(id models.BlockID) (block *Block, exists bool) {
	if s.EvictionManager.IsRootBlock(id) {
		tangleBlock, _ := s.Tangle.Block(id)

		return NewBlock(tangleBlock), true
	}

	storage := s.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

func (s *Scheduler) getOrRegisterBlock(virtualVotingBlock *virtualvoting.Block) (block *Block, err error) {
	if s.EvictionManager.IsTooOld(virtualVotingBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted epoch", virtualVotingBlock.ID())
	}
	block, exists := s.block(virtualVotingBlock.ID())
	if exists {
		return block, nil
	}

	blockStorage := s.blocks.Get(virtualVotingBlock.ID().Index(), true)

	block, _ = blockStorage.RetrieveOrCreate(virtualVotingBlock.ID(), func() *Block {
		return NewBlock(virtualVotingBlock)
	})

	return block, nil
}
