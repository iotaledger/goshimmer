package scheduler

import (
	"fmt"
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/atomic"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/memstorage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/runtime/options"
)

const (
	// MinMana is the minimum amount of Mana needed to issue blocks.
	// MaxBlockSize / MinMana is also the upper bound of iterations inside one schedule call, as such it should not be too small.
	MinMana int64 = 1
)

// ErrNotRunning is returned when a block is submitted when the scheduler has been stopped.
var ErrNotRunning = errors.New("scheduler stopped")

// region Scheduler ////////////////////////////////////////////////////////////////////////////////////////////////////

// Scheduler is a Tangle component that takes care of scheduling the blocks that shall be booked.
type Scheduler struct {
	Events *Events

	slotTimeProvider *slot.TimeProvider

	blocks        *memstorage.SlotStorage[models.BlockID, *Block]
	bufferMutex   sync.RWMutex
	buffer        *BufferQueue
	deficitsMutex sync.RWMutex
	deficits      *shrinkingmap.ShrinkingMap[identity.ID, *big.Rat]
	evictionState *eviction.State
	evictionMutex sync.RWMutex

	totalAccessManaRetrieveFunc func() int64
	accessManaMapRetrieverFunc  func() map[identity.ID]int64
	isBlockAcceptedFunc         func(models.BlockID) bool

	optsRate                           time.Duration
	optsMaxBufferSize                  int
	optsAcceptedBlockScheduleThreshold time.Duration
	optsMaxDeficit                     *big.Rat

	running        atomic.Bool
	shutdownSignal chan struct{}
}

// New returns a new Scheduler.
func New(evictionState *eviction.State, slotTimeProvider *slot.TimeProvider, isBlockAccepted func(models.BlockID) bool, accessManaMapRetrieverFunc func() map[identity.ID]int64, totalAccessManaRetrieveFunc func() int64, opts ...options.Option[Scheduler]) *Scheduler {
	return options.Apply(&Scheduler{
		Events: NewEvents(),

		evictionState:               evictionState,
		slotTimeProvider:            slotTimeProvider,
		isBlockAcceptedFunc:         isBlockAccepted,
		accessManaMapRetrieverFunc:  accessManaMapRetrieverFunc,
		totalAccessManaRetrieveFunc: totalAccessManaRetrieveFunc,

		deficits:                           shrinkingmap.New[identity.ID, *big.Rat](),
		blocks:                             memstorage.NewSlotStorage[models.BlockID, *Block](),
		optsMaxBufferSize:                  300,
		optsAcceptedBlockScheduleThreshold: 5 * time.Minute,
		optsRate:                           5 * time.Millisecond,      // measured in time per unit work
		optsMaxDeficit:                     new(big.Rat).SetInt64(10), // must be >= max block work, but work is currently=1 for all blocks
	}, opts, func(s *Scheduler) {
		s.buffer = NewBufferQueue(s.optsMaxBufferSize)
	}, (*Scheduler).setupEvents)
}

func (s *Scheduler) setupEvents() {
	s.Events.BlockScheduled.Hook(s.UpdateChildren)
	s.evictionState.Events.SlotEvicted.Hook(s.evictSlot)
}

// Start starts the scheduler.
func (s *Scheduler) Start() {
	if s.running.CompareAndSwap(false, true) {
		s.shutdownSignal = make(chan struct{}, 1)
		// start the main loop
		go s.mainLoop()
	}
}

// IsRunning returns true if the scheduler has started.
func (s *Scheduler) IsRunning() bool {
	return s.running.Load()
}

// Rate gets the rate of the scheduler.
func (s *Scheduler) Rate() time.Duration {
	return s.optsRate
}

// IsUncongested checks if the issuerQueue is completely uncongested.
func (s *Scheduler) IsUncongested() bool {
	return s.ReadyBlocksCount() == 0
}

// IssuerQueueSize returns the size of the IssuerIDs queue in number of blocks.
func (s *Scheduler) IssuerQueueSize(issuerID identity.ID) int {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
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
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
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
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.MaxSize()
}

// BufferSize returns the number of blocks in the buffer.
func (s *Scheduler) BufferSize() int {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.Size()
}

// ReadyBlocksCount returns the number of blocks that are ready to be scheduled.
func (s *Scheduler) ReadyBlocksCount() int {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.ReadyBlocksCount()
}

// TotalBlocksCount returns the size of the buffer in number of blocks.
func (s *Scheduler) TotalBlocksCount() int {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()

	return s.buffer.TotalBlocksCount()
}

func (s *Scheduler) Quanta(issuerID identity.ID) *big.Rat {
	return quanta(issuerID, s.accessManaMapRetrieverFunc(), s.totalAccessManaRetrieveFunc())
}

func (s *Scheduler) Deficit(issuerID identity.ID) *big.Rat {
	s.deficitsMutex.RLock()
	defer s.deficitsMutex.RUnlock()

	deficit, exists := s.deficits.Get(issuerID)
	if !exists {
		return new(big.Rat).SetInt64(0)
	}

	return deficit
}

func (s *Scheduler) GetAccessManaMap() map[identity.ID]int64 {
	return s.accessManaMapRetrieverFunc()
}

// Shutdown shuts down the Scheduler.
// Shutdown blocks until the scheduler has been shutdown successfully.
func (s *Scheduler) Shutdown() {
	if s.running.CompareAndSwap(true, false) {
		close(s.shutdownSignal)
	}
}

// Block retrieves the Block with given id from the mem-storage.
func (s *Scheduler) Block(id models.BlockID) (block *Block, exists bool) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	return s.block(id)
}

func (s *Scheduler) AddBlock(sourceBlock *booker.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	block, _ := s.GetOrRegisterBlock(sourceBlock)

	if block.IsOrphaned() {
		if block.SetDropped() {
			s.Events.BlockDropped.Trigger(block)
		}

		return
	}

	if err := s.Submit(block); err != nil {
		if !errors.Is(err, ErrInsufficientMana) {
			s.Events.Error.Trigger(errors.Wrap(err, "failed to submit to scheduler"))
		}
	}
	s.tryReady(block)
}

func (s *Scheduler) HandleOrphanedBlock(orphanedBlock *blockdag.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	block, exists := s.block(orphanedBlock.ID())
	if !exists || block.IsDropped() || block.IsSkipped() || block.IsScheduled() {
		return
	}

	s.Unsubmit(block)
	if block.SetDropped() {
		s.Events.BlockDropped.Trigger(block)
	}
}

func (s *Scheduler) HandleAcceptedBlock(acceptedBlock *blockgadget.Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()

	block, err := s.GetOrRegisterBlock(acceptedBlock.Block)
	if err != nil {
		s.Events.Error.Trigger(errors.Wrap(err, "failed to get or register block"))
		return
	}

	if block.IsScheduled() || block.IsDropped() || block.IsSkipped() {
		return
	}

	if time.Since(block.IssuingTime()) > s.optsAcceptedBlockScheduleThreshold {
		s.Unsubmit(block)
		block.SetSkipped()
		s.Events.BlockSkipped.Trigger(block)
	}

	s.updateChildren(block)
}

// UpdateChildren iterates over the direct children of the given blockID and
// tries to mark them as ready.
func (s *Scheduler) UpdateChildren(block *Block) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
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

	if err := s.submit(block); err != nil {
		return err
	}

	s.ready(block)

	return nil
}

// isEligible returns true if the given blockID has either been (scheduled and not orphaned) or confirmed.
// Check if parent is orphaned should not be necessary, because if block's parent is orphaned,
// then block itself should be orphaned as well, but leaving it here for safety.
func (s *Scheduler) isEligible(block *Block) (eligible bool) {
	return (block.IsScheduled() && !block.IsOrphaned()) || s.isBlockAcceptedFunc(block.ID())
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
	if !s.IsRunning() {
		return ErrNotRunning
	}

	// TODO: when removing the zero mana issuer solution, check if issuers have MinMana here
	droppedBlocks, err := s.buffer.Submit(block, s.getAccessMana)
	if err != nil {
		return errors.Wrapf(err, "failed to submit %s", block.ID())
	}

	s.markAsDropped(droppedBlocks)

	s.Events.BlockSubmitted.Trigger(block)

	return nil
}

func (s *Scheduler) markAsDropped(droppedBlocks []*Block) {
	for _, droppedBlock := range droppedBlocks {
		if droppedBlock.SetDropped() {
			s.Events.BlockDropped.Trigger(droppedBlock)
		}
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
	if s.evictionState.IsRootBlock(id) {
		return NewRootBlock(id, s.slotTimeProvider), true
	}

	storage := s.blocks.Get(id.Index(), false)
	if storage == nil {
		return nil, false
	}

	return storage.Get(id)
}

// mainLoop periodically triggers the scheduling of ready blocks.
func (s *Scheduler) mainLoop() {
	ticker := time.NewTicker(s.optsRate)
	defer ticker.Stop()

loop:
	for {
		select {
		// on close, exit the loop
		case <-s.shutdownSignal:
			break loop
		// every rate time units
		case <-ticker.C:
			if !s.IsRunning() {
				break loop
			}
			if block := s.schedule(); block != nil {
				// TODO: make this operate in units of work, with a variable pause between scheduling depending on work scheduled.
				// TODO: don't use a ticker. Switch to a simple timer instead, and use a flag when ready to schedule something if there is nothing ready to be scheduled yet.
				// TODO: implement a token bucket for the scheduler to account for bursty arrivals.
				if block.SetScheduled() {
					s.Events.BlockScheduled.Trigger(block)
				}
			}
		}
	}
}

func (s *Scheduler) schedule() *Block {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()

	manaMap := s.accessManaMapRetrieverFunc()
	totalMana := s.totalAccessManaRetrieveFunc()

	s.updateActiveIssuersList(manaMap)

	start := s.buffer.Current()
	// no blocks submitted
	if start == nil {
		return nil
	}

	rounds, schedulingIssuer := s.selectIssuer(start, manaMap, totalMana)

	// if there is no issuer with a ready block, we cannot schedule anything
	if schedulingIssuer == nil {
		return nil
	}

	if rounds.Sign() > 0 {
		// increment every issuer's deficit for the required number of rounds
		for q := start; ; {
			s.updateDeficit(q.IssuerID(), new(big.Rat).Mul(quanta(q.IssuerID(), manaMap, totalMana), rounds))

			q = s.buffer.Next()
			if q == start {
				break
			}
		}
	}

	// increment the deficit for all issuers before schedulingIssuer one more time
	for q := start; q != schedulingIssuer; q = s.buffer.Next() {
		s.updateDeficit(q.IssuerID(), quanta(q.IssuerID(), manaMap, totalMana))
	}

	// remove the block from the buffer and adjust issuer's deficit
	block := s.buffer.PopFront()
	issuerID := identity.NewID(block.IssuerPublicKey())
	s.updateDeficit(issuerID, new(big.Rat).SetInt64(-int64(block.Work())))
	return block
}

func (s *Scheduler) selectIssuer(start *IssuerQueue, manaMap map[identity.ID]int64, totalMana int64) (rounds *big.Rat, schedulingIssuer *IssuerQueue) {
	rounds = new(big.Rat).SetFloat64(math.MaxFloat64)

	for q := start; ; {
		block := q.Front()

		for block != nil && time.Now().After(block.IssuingTime()) {
			if s.isBlockAcceptedFunc(block.ID()) && time.Since(block.IssuingTime()) > s.optsAcceptedBlockScheduleThreshold {
				block.SetSkipped()
				s.Events.BlockSkipped.Trigger(block)
				s.buffer.PopFront()

				block = q.Front()

				continue
			}

			if block.IsDropped() || block.IsOrphaned() {
				s.buffer.PopFront()
				block = q.Front()

				continue
			}

			// compute how often the deficit needs to be incremented until the block can be scheduled
			remainingDeficit := maxRat(new(big.Rat).Sub(big.NewRat(int64(block.Work()), 1), s.Deficit(q.IssuerID())), new(big.Rat))
			// calculate how many rounds we need to skip to accumulate enough deficit.
			// Use for loop to account for float imprecision.
			r := new(big.Rat).Mul(remainingDeficit, new(big.Rat).Inv(quanta(q.IssuerID(), manaMap, totalMana)))
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

func (s *Scheduler) updateActiveIssuersList(manaMap map[identity.ID]int64) {
	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	// remove issuers that don't have mana and have empty queue
	// this allows issuers with zero mana to issue blocks, however those issuers will only accumulate their deficit
	// when there are blocks in the issuer's queue
	currentIssuer := s.buffer.Current()
	numIssuers := s.buffer.NumActiveIssuers()
	for i := 0; i < numIssuers; i++ {
		if issuerMana, exists := manaMap[currentIssuer.IssuerID()]; (!exists || issuerMana < MinMana) && currentIssuer.Work() == 0 {
			s.buffer.RemoveIssuer(currentIssuer.IssuerID())
			s.deficits.Delete(currentIssuer.IssuerID())

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

		if _, exists := s.deficits.Get(issuerID); !exists {
			s.deficits.Set(issuerID, new(big.Rat).SetInt64(0))
			s.buffer.InsertIssuer(issuerID)
		}
	}
}

func (s *Scheduler) updateDeficit(issuerID identity.ID, d *big.Rat) {
	deficit := new(big.Rat).Add(s.Deficit(issuerID), d)
	if deficit.Sign() < 0 {
		// this will never happen and is just here for debugging purposes
		panic("scheduler: deficit is less than 0")
	}

	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	s.deficits.Set(issuerID, minRat(deficit, s.optsMaxDeficit))
}

func (s *Scheduler) GetExcessDeficit(issuerID identity.ID) (deficitFloat float64, err error) {
	s.evictionMutex.RLock()
	defer s.evictionMutex.RUnlock()
	s.bufferMutex.RLock()
	defer s.bufferMutex.RUnlock()
	s.deficitsMutex.RLock()
	defer s.deficitsMutex.RUnlock()

	if deficit, exists := s.deficits.Get(issuerID); exists {
		deficitFloat, _ = deficit.Float64()
		return deficitFloat - float64(s.issuerQueueWork(issuerID)), nil
	}

	return 0.0, errors.Errorf("Deficit for issuer %s does not exist", issuerID)
}

func (s *Scheduler) GetOrRegisterBlock(virtualVotingBlock *booker.Block) (block *Block, err error) {
	if s.evictionState.InEvictedSlot(virtualVotingBlock.ID()) {
		return nil, errors.Errorf("block %s belongs to an evicted slot", virtualVotingBlock.ID())
	}
	block, exists := s.block(virtualVotingBlock.ID())
	if exists {
		return block, nil
	}

	blockStorage := s.blocks.Get(virtualVotingBlock.ID().Index(), true)

	block, _ = blockStorage.GetOrCreate(virtualVotingBlock.ID(), func() *Block {
		return NewBlock(virtualVotingBlock)
	})

	return block, nil
}

func (s *Scheduler) issuerQueueWork(issuerID identity.ID) int {
	issuerQueue := s.buffer.IssuerQueue(issuerID)
	if issuerQueue == nil {
		return 0
	}
	return issuerQueue.Work()
}

func (s *Scheduler) getAccessMana(id identity.ID) int64 {
	mana := s.accessManaMapRetrieverFunc()[id]
	if mana == 0 {
		// TODO: when removing zero mana issuers, remove this
		return MinMana
	}
	return mana
}

func (s *Scheduler) evictSlot(index slot.Index) {
	s.evictionMutex.Lock()
	defer s.evictionMutex.Unlock()

	// TODO: this can be implemented better, but usually there will not be a lot of blocks in the buffer anyway, so it's good enough for a quickfix
	// A possible problem is that a block is stuck in the buffer, without ever being marked as ready.
	// This situation should never occur, but we suspect it does happen sometimes and the following fix should remedy that.
	// Probably it can be deleted once the original problem is fixed.
	// If a block is marked as Ready, then it will be eventually removed by the scheduling loop.
	for _, blockID := range s.buffer.IDs() {
		if blockID.Index() == index {
			if block, exists := s.block(blockID); exists {
				s.buffer.Unsubmit(block)
				fmt.Printf(">> Scheduler evicted block %s from the buffer when evicting slot %s\n", blockID.String(), index.String())
			}
		}
	}

	s.blocks.Evict(index)
}

func quanta(issuerID identity.ID, manaMap map[identity.ID]int64, totalMana int64) *big.Rat {
	if manaValue, exists := manaMap[issuerID]; exists {
		if manaValue == 0 {
			manaValue = MinMana
		}
		return big.NewRat(manaValue, totalMana)
	}

	// TODO: when removing the zero mana issuer solution, return zero
	return big.NewRat(MinMana, totalMana)
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

func WithRate(rate time.Duration) options.Option[Scheduler] {
	return func(s *Scheduler) {
		s.optsRate = rate
	}
}

func WithMaxDeficit(maxDef int) options.Option[Scheduler] {
	return func(s *Scheduler) {
		s.optsMaxDeficit = new(big.Rat).SetInt64(int64(maxDef))
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
