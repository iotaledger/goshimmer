package scheduler

import (
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/typeutils"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/core/clock"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/memstorage"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/acceptance"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
)

const (
	// MinMana is the minimum amount of Mana needed to issue blocks.
	// MaxBlockSize / MinMana is also the upper bound of iterations inside one schedule call, as such it should not be too small.
	MinMana float64 = 1.0
)

// MaxDeficit is the maximum cap for accumulated deficit, i.e. max bytes that can be scheduled without waiting.
// It must be >= MaxBlockSize.
var MaxDeficit = new(big.Rat).SetInt64(int64(models.MaxBlockSize))

// ErrNotRunning is returned when a block is submitted when the scheduler has been stopped.
var ErrNotRunning = errors.New("scheduler stopped")

// region Scheduler ////////////////////////////////////////////////////////////////////////////////////////////////////

// Scheduler is a Tangle component that takes care of scheduling the blocks that shall be booked.
type Scheduler struct {
	Events *Events
	Tangle *tangle.Tangle

	EvictionManager *eviction.LockableManager[models.BlockID]

	blocks        *memstorage.EpochStorage[models.BlockID, *Block]
	ticker        *time.Ticker
	bufferMutex   sync.RWMutex
	buffer        *BufferQueue
	deficitsMutex sync.RWMutex
	deficits      map[identity.ID]*big.Rat

	totalAccessManaRetrieveFunc func() float64
	accessManaMapRetrieverFunc  func() map[identity.ID]float64
	isBlockAcceptedFunc         func(models.BlockID) bool
	blockAcceptedEvent          *event.Event[*acceptance.Block]

	rate                               *atomic.Duration
	optsMaxBufferSize                  int
	optsAcceptedBlockScheduleThreshold time.Duration

	started        typeutils.AtomicBool
	stopped        typeutils.AtomicBool
	shutdownSignal chan struct{}
	shutdownOnce   sync.Once
}

// New returns a new Scheduler.
func New(isBlockAccepted func(models.BlockID) bool, blockAcceptedEvent *event.Event[*acceptance.Block], tangle *tangle.Tangle, accessManaMapRetrieverFunc func() map[identity.ID]float64, totalAccessManaRetrieveFunc func() float64, rate time.Duration, opts ...options.Option[Scheduler]) *Scheduler {
	return options.Apply(&Scheduler{
		Events:          newEvents(),
		Tangle:          tangle,
		EvictionManager: tangle.EvictionManager.Lockable(),

		isBlockAcceptedFunc:         isBlockAccepted,
		blockAcceptedEvent:          blockAcceptedEvent,
		accessManaMapRetrieverFunc:  accessManaMapRetrieverFunc,
		totalAccessManaRetrieveFunc: totalAccessManaRetrieveFunc,
		rate:                        atomic.NewDuration(rate),

		deficits:                           make(map[identity.ID]*big.Rat),
		blocks:                             memstorage.NewEpochStorage[models.BlockID, *Block](),
		optsMaxBufferSize:                  300,
		optsAcceptedBlockScheduleThreshold: 5 * time.Minute,

		shutdownSignal: make(chan struct{}),
	}, opts, func(s *Scheduler) {
		// maximum access mana-scaled inbox length
		s.ticker = time.NewTicker(s.rate.Load())
		s.buffer = NewBufferQueue(s.optsMaxBufferSize)

	}, (*Scheduler).setupEvents)

}

func (s *Scheduler) setupEvents() {
	// pass booked blocks to the scheduler
	s.Tangle.VirtualVoting.Events.BlockTracked.Attach(event.NewClosure(s.AddBlock))

	s.Events.BlockScheduled.Hook(event.NewClosure(s.UpdateChildren))

	s.blockAcceptedEvent.Attach(event.NewClosure(s.HandleAcceptedBlock))

	s.EvictionManager.Events.EpochEvicted.Attach(event.NewClosure(s.evictEpoch))

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
	return s.rate.Load()
}

// IssuerQueueSize returns the size of the IssuerIDs queue.
func (s *Scheduler) IssuerQueueSize(issuerID identity.ID) int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	issuerQueue := s.buffer.IssuerQueue(issuerID)
	if issuerQueue == nil {
		return 0
	}
	return issuerQueue.Size()
}

// IssuerQueueSizes returns the size for each issuer queue.
func (s *Scheduler) IssuerQueueSizes() map[identity.ID]int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	queueSizes := make(map[identity.ID]int)
	for _, issuerID := range s.buffer.IssuerIDs() {
		size := s.buffer.IssuerQueue(issuerID).Size()
		queueSizes[issuerID] = size
	}
	return queueSizes
}

// MaxBufferSize returns the max size of the buffer.
func (s *Scheduler) MaxBufferSize() int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.MaxSize()
}

// BufferSize returns the size of the buffer.
func (s *Scheduler) BufferSize() int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.Size()
}

// ReadyBlocksCount returns the size buffer.
func (s *Scheduler) ReadyBlocksCount() int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.ReadyBlocksCount()
}

// TotalBlocksCount returns the size buffer.
func (s *Scheduler) TotalBlocksCount() int {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.TotalBlocksCount()
}

func (s *Scheduler) Quanta(issuerID identity.ID) *big.Rat {
	return big.NewRat(int64(s.getAccessMana(issuerID)), int64(s.totalAccessManaRetrieveFunc()))
}

func (s *Scheduler) Deficit(issuerID identity.ID) *big.Rat {
	s.deficitsMutex.RLock()
	defer s.deficitsMutex.RUnlock()

	deficit, exists := s.deficits[issuerID]
	if !exists {
		return new(big.Rat).SetInt64(0)
	}

	return deficit
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	s.shutdownOnce.Do(func() {
		s.stopped.Set()
		s.shutdownSignal <- struct{}{}
		close(s.shutdownSignal)
	})
}

// Block retrieves the Block with given id from the mem-storage.
func (s *Scheduler) Block(id models.BlockID) (block *Block, exists bool) {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()

	return s.block(id)
}

func (s *Scheduler) AddBlock(sourceBlock *virtualvoting.Block) {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()

	block, _ := s.getOrRegisterBlock(sourceBlock)

	if err := s.Submit(block); err != nil {
		if !errors.Is(err, ErrInsufficientMana) {
			s.Events.Error.Trigger(errors.Wrap(err, "failed to submit to scheduler"))
		}
	}
	s.tryReady(block)
}

func (s *Scheduler) HandleAcceptedBlock(acceptedBlock *acceptance.Block) {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()

	block, _ := s.getOrRegisterBlock(acceptedBlock.Block)

	scheduled := block.Scheduled()
	if scheduled {
		return
	}

	if clock.Since(block.IssuingTime()) > s.optsAcceptedBlockScheduleThreshold {
		s.Unsubmit(block)
		block.SetSkipped()
		s.Events.BlockSkipped.Trigger(block)
	}

	s.updateChildren(block)
}

// UpdateChildren iterates over the direct children of the given blockID and
// tries to mark them as ready.
func (s *Scheduler) UpdateChildren(block *Block) {
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()
	s.updateChildren(block)
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
	return block.Scheduled() || s.isBlockAcceptedFunc(block.ID())
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

	// TODO: when removing the zero mana issuer solution, check if issuers have MinMana here
	droppedBlocks, err := s.buffer.Submit(block, s.getAccessMana)
	if err != nil {
		return errors.Wrapf(err, "failed to submit %s", block.ID())
	}

	s.markAsDropped(droppedBlocks)

	return nil
}

func (s *Scheduler) markAsDropped(droppedBlocks []*Block) {
	for _, droppedBlock := range droppedBlocks {
		s.Tangle.SetOrphaned(droppedBlock.Block.Block.Block, true)
		s.Events.BlockDropped.Trigger(droppedBlock)
	}
}

func (s *Scheduler) unsubmit(block *Block) {
	s.buffer.Unsubmit(block)
}

func (s *Scheduler) ready(block *Block) {
	s.buffer.Ready(block)
}

// block retrieves the Block with given id from the mem-storage.
func (s *Scheduler) block(id models.BlockID) (block *Block, exists bool) {
	if s.EvictionManager.IsRootBlock(id) {
		tangleBlock, _ := s.Tangle.Block(id)

		return NewBlock(tangleBlock, WithScheduled(true)), true
	}

	storage := s.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
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

func (s *Scheduler) schedule() *Block {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()
	s.EvictionManager.RLock()
	defer s.EvictionManager.RUnlock()

	s.updateActiveIssuersList(s.accessManaMapRetrieverFunc())

	start := s.buffer.Current()
	// no blocks submitted
	if start == nil {
		return nil
	}

	rounds, schedulingIssuer := s.selectIssuer(start)

	// if there is no issuer with a ready block, we cannot schedule anything
	if schedulingIssuer == nil {
		return nil
	}

	if rounds.Sign() > 0 {
		// increment every issuer's deficit for the required number of rounds
		for q := start; ; {
			s.updateDeficit(q.IssuerID(), new(big.Rat).Mul(s.Quanta(q.IssuerID()), rounds))

			q = s.buffer.Next()
			if q == start {
				break
			}
		}
	}

	// increment the deficit for all issuers before schedulingIssuer one more time
	for q := start; q != schedulingIssuer; q = s.buffer.Next() {
		s.updateDeficit(q.IssuerID(), s.Quanta(q.IssuerID()))
	}

	// remove the block from the buffer and adjust issuer's deficit
	block := s.buffer.PopFront()
	issuerID := identity.NewID(block.IssuerPublicKey())
	s.updateDeficit(issuerID, new(big.Rat).SetInt64(-int64(block.Size())))

	return block
}

func (s *Scheduler) selectIssuer(start *IssuerQueue) (rounds *big.Rat, schedulingIssuer *IssuerQueue) {
	rounds = new(big.Rat).SetInt64(math.MaxInt64)

	for q := start; ; {
		block := q.Front()

		for block != nil && !clock.SyncedTime().Before(block.IssuingTime()) {
			if s.isBlockAcceptedFunc(block.ID()) && clock.Since(block.IssuingTime()) > s.optsAcceptedBlockScheduleThreshold {
				block.SetSkipped()
				s.Events.BlockSkipped.Trigger(block)
				s.buffer.PopFront()

				block = q.Front()

				continue
			}

			// compute how often the deficit needs to be incremented until the block can be scheduled
			remainingDeficit := maxRat(new(big.Rat).Sub(big.NewRat(int64(block.Size()), 1), s.Deficit(q.IssuerID())), new(big.Rat))
			// calculate how many rounds we need to skip to accumulate enough deficit.
			// Use for loop to account for float imprecision.
			r := new(big.Rat).Mul(remainingDeficit, new(big.Rat).Inv(s.Quanta(q.IssuerID())))
			// find the first issuer that will be allowed to schedule a block
			if r.Cmp(rounds) < 0 {
				rounds = r
				schedulingIssuer = q
			}
			break
		}

		q = s.buffer.Next()
		if q == start {
			break
		}
	}
	return rounds, schedulingIssuer
}

func (s *Scheduler) updateActiveIssuersList(manaMap map[identity.ID]float64) {
	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	// remove issuers that don't have mana and have empty queue
	// this allows issuers with zero mana to issue blocks, however those issuers will only accumulate their deficit
	// when there are blocks in the issuer's queue
	currentIssuer := s.buffer.Current()
	numIssuers := s.buffer.NumActiveIssuers()
	for i := 0; i < numIssuers; i++ {
		if issuerMana, exists := manaMap[currentIssuer.IssuerID()]; (!exists || issuerMana < MinMana) && currentIssuer.Size() == 0 {
			s.buffer.RemoveIssuer(currentIssuer.IssuerID())
			delete(s.deficits, currentIssuer.IssuerID())
			currentIssuer = s.buffer.Current()
		} else {
			currentIssuer = s.buffer.Next()
		}
	}

	// update list of active issuers with accumulating deficit
	for issuerID, issuerMana := range manaMap {
		if issuerMana < MinMana {
			continue
		}

		if _, exists := s.deficits[issuerID]; !exists {
			s.deficits[issuerID] = new(big.Rat).SetInt64(0)
			s.buffer.InsertIssuer(issuerID)
		}
	}
}

func (s *Scheduler) updateDeficit(issuerID identity.ID, d *big.Rat) {
	deficit := new(big.Rat).Add(s.Deficit(issuerID), d)
	if deficit.Sign() < 0 {
		// this will never happen and is just here for debugging purposes
		// TODO: remove print
		panic("scheduler: deficit is less than 0")
	}

	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	s.deficits[issuerID] = minRat(deficit, MaxDeficit)
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

func (s *Scheduler) getAccessMana(id identity.ID) float64 {
	mana, exists := s.accessManaMapRetrieverFunc()[id]
	if exists {
		return mana
	}
	return 0.0
}

func (s *Scheduler) evictEpoch(index epoch.Index) {
	s.EvictionManager.Lock()
	defer s.EvictionManager.Unlock()

	s.blocks.EvictEpoch(index)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options ////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptedBlockScheduleThreshold(acceptedBlockScheduleThreshold time.Duration) options.Option[Scheduler] {
	return func(s *Scheduler) {
		s.optsAcceptedBlockScheduleThreshold = acceptedBlockScheduleThreshold
	}
}

func WithMaxBufferSize(maxBufferSize int) options.Option[Scheduler] {
	return func(s *Scheduler) {
		s.optsMaxBufferSize = maxBufferSize
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func minRat(x, y *big.Rat) *big.Rat {
	if x.Cmp(y) < 0 {
		return x
	}
	return y
}

func maxRat(x, y *big.Rat) *big.Rat {
	if x.Cmp(y) > 0 {
		return x
	}
	return y
}
