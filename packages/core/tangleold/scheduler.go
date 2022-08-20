package tangleold

import (
	"math"
	"math/big"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/typeutils"
	"go.uber.org/atomic"

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

// SchedulerParams defines the scheduler config parameters.
type SchedulerParams struct {
	MaxBufferSize                   int
	TotalSupply                     int
	Rate                            time.Duration
	TotalAccessManaRetrieveFunc     func() float64
	AccessManaMapRetrieverFunc      func() map[identity.ID]float64
	ConfirmedBlockScheduleThreshold time.Duration
}

// Scheduler is a Tangle component that takes care of scheduling the blocks that shall be booked.
type Scheduler struct {
	Events *SchedulerEvents

	tangle                *Tangle
	ticker                *time.Ticker
	started               typeutils.AtomicBool
	stopped               typeutils.AtomicBool
	accessManaCache       *schedulerutils.AccessManaCache
	bufferMutex           sync.RWMutex
	buffer                *schedulerutils.BufferQueue
	deficitsMutex         sync.RWMutex
	deficits              map[identity.ID]*big.Rat
	rate                  *atomic.Duration
	confirmedBlkThreshold time.Duration
	shutdownSignal        chan struct{}
	shutdownOnce          sync.Once
}

// NewScheduler returns a new Scheduler.
func NewScheduler(tangle *Tangle) *Scheduler {
	if tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc == nil || tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc == nil {
		panic("scheduler: the option AccessManaMapRetrieverFunc and AccessManaRetriever and TotalAccessManaRetriever must be defined so that AccessMana can be determined in scheduler")
	}

	// maximum buffer size (in bytes)
	maxBuffer := tangle.Options.SchedulerParams.MaxBufferSize

	// total supply of mana
	totalSupply := tangle.Options.SchedulerParams.TotalSupply

	// threshold after which confirmed blocks are not scheduled
	confirmedBlockScheduleThreshold := tangle.Options.SchedulerParams.ConfirmedBlockScheduleThreshold

	// maximum access mana-scaled inbox length
	maxQueue := float64(maxBuffer) / float64(totalSupply)

	accessManaCache := schedulerutils.NewAccessManaCache(
		tangle.Options.SchedulerParams.AccessManaMapRetrieverFunc,
		tangle.Options.SchedulerParams.TotalAccessManaRetrieveFunc,
		MinMana,
	)

	return &Scheduler{
		Events:                NewSchedulerEvents(),
		tangle:                tangle,
		accessManaCache:       accessManaCache,
		rate:                  atomic.NewDuration(tangle.Options.SchedulerParams.Rate),
		ticker:                time.NewTicker(tangle.Options.SchedulerParams.Rate),
		buffer:                schedulerutils.NewBufferQueue(maxBuffer, maxQueue),
		confirmedBlkThreshold: confirmedBlockScheduleThreshold,
		deficits:              make(map[identity.ID]*big.Rat),
		shutdownSignal:        make(chan struct{}),
	}
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

// Setup sets up the behavior of the component by making it attach to the relevant events of the other components.
func (s *Scheduler) Setup() {
	// pass booked blocks to the scheduler
	s.tangle.ApprovalWeightManager.Events.BlockProcessed.Attach(event.NewClosure(func(event *BlockProcessedEvent) {
		if err := s.Submit(event.BlockID); err != nil {
			if !errors.Is(err, schedulerutils.ErrInsufficientMana) {
				s.Events.Error.Trigger(errors.Errorf("failed to submit to scheduler: %w", err))
			}
		}
		s.tryReady(event.BlockID)
	}))

	s.tangle.Scheduler.Events.BlockScheduled.Hook(event.NewClosure(func(event *BlockScheduledEvent) {
		s.updateChildren(event.BlockID)
	}))

	onBlockConfirmed := func(block *Block) {
		blockID := block.ID()
		var scheduled bool
		s.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
			scheduled = blockMetadata.Scheduled()
		})
		if scheduled {
			return
		}
		s.tangle.Storage.Block(blockID).Consume(func(block *Block) {
			if clock.Since(block.IssuingTime()) > s.confirmedBlkThreshold {
				err := s.Unsubmit(blockID)
				if err != nil {
					s.Events.Error.Trigger(errors.Errorf("failed to unsubmit confirmed block from scheduler: %w", err))
				}
				s.Events.BlockSkipped.Trigger(&BlockSkippedEvent{blockID})
			}
		})
		s.updateChildren(blockID)
	}

	s.tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(event *BlockAcceptedEvent) {
		onBlockConfirmed(event.Block)
	}))

	s.Start()
}

// SetRate sets the rate of the scheduler.
func (s *Scheduler) SetRate(rate time.Duration) {
	// only update the ticker when the scheduler is running
	if !s.stopped.IsSet() {
		s.ticker.Reset(rate)
		s.rate.Store(rate)
	}
}

// Rate gets the rate of the scheduler.
func (s *Scheduler) Rate() time.Duration {
	return s.rate.Load()
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

// AccessManaCache returns the object which caches access mana values.
func (s *Scheduler) AccessManaCache() *schedulerutils.AccessManaCache {
	return s.accessManaCache
}

// Submit submits a block to be considered by the scheduler.
// This transactions will be included in all the control metrics, but it will never be
// scheduled until Ready(blockID) has been called.
func (s *Scheduler) Submit(blockID BlockID) (err error) {
	if !s.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		s.bufferMutex.Lock()
		defer s.bufferMutex.Unlock()
		err = s.submit(block)
	}) {
		err = errors.Errorf("failed to get block '%x' from storage", blockID)
	}
	return err
}

// Unsubmit removes a block from the submitted blocks.
// If that block is already marked as ready, Unsubmit has no effect.
func (s *Scheduler) Unsubmit(blockID BlockID) (err error) {
	if !s.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		s.bufferMutex.Lock()
		defer s.bufferMutex.Unlock()

		s.unsubmit(block)
	}) {
		err = errors.Errorf("failed to get block '%x' from storage", blockID)
	}
	return err
}

// Ready marks a previously submitted block as ready to be scheduled.
// If Ready is called without a previous Submit, it has no effect.
func (s *Scheduler) Ready(blockID BlockID) (err error) {
	if !s.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		s.bufferMutex.Lock()
		defer s.bufferMutex.Unlock()

		s.ready(block)
	}) {
		err = errors.Errorf("failed to get block '%x' from storage", blockID)
	}
	return err
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

// GetManaFromCache allows you to get the cached mana for a node ID. This is exposed for analytics purposes.
func (s *Scheduler) GetManaFromCache(nodeID identity.ID) int64 {
	return int64(math.Ceil(s.AccessManaCache().GetCachedMana(nodeID)))
}

// isEligible returns true if the given blockID has either been scheduled or confirmed.
func (s *Scheduler) isEligible(blockID BlockID) (eligible bool) {
	s.tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *BlockMetadata) {
		eligible = blockMetadata.Scheduled() ||
			s.tangle.ConfirmationOracle.IsBlockConfirmed(blockID)
	})
	return
}

// isReady returns true if the given blockID's parents are eligible.
func (s *Scheduler) isReady(blockID BlockID) (ready bool) {
	ready = true
	s.tangle.Storage.Block(blockID).Consume(func(block *Block) {
		block.ForEachParent(func(parent Parent) {
			if !s.isEligible(parent.ID) { // parents are not eligible
				ready = false
				return
			}
		})
	})

	return
}

// tryReady tries to set the given block as ready.
func (s *Scheduler) tryReady(blockID BlockID) {
	if s.isReady(blockID) {
		if err := s.Ready(blockID); err != nil {
			s.Events.Error.Trigger(errors.Errorf("failed to mark %s as ready: %w", blockID, err))
		}
	}
}

// updateChildren iterates over the direct children of the given blockID and
// tries to mark them as ready.
func (s *Scheduler) updateChildren(blockID BlockID) {
	s.tangle.Storage.Children(blockID).Consume(func(child *Child) {
		s.tryReady(child.ChildBlockID())
	})
}

func (s *Scheduler) submit(block *Block) error {
	if s.stopped.IsSet() {
		return ErrNotRunning
	}

	s.tangle.Storage.BlockMetadata(block.ID()).Consume(func(blockMetadata *BlockMetadata) {
		// shortly before submitting we set the queued time
		blockMetadata.SetQueuedTime(clock.SyncedTime())
	})

	s.AccessManaCache().RefreshCacheIfNecessary()
	// when removing the zero mana node solution, check if nodes have MinMana here
	droppedBlockIDs, err := s.buffer.Submit(block, s.AccessManaCache().GetCachedMana)
	if err != nil {
		panic(errors.Errorf("failed to submit %s: %w", block.ID(), err))
	}
	for _, droppedBlkID := range droppedBlockIDs {
		s.tangle.Storage.BlockMetadata(blockIDFromElementID(droppedBlkID)).Consume(func(blockMetadata *BlockMetadata) {
			blockMetadata.SetDiscardedTime(clock.SyncedTime())
		})
		s.Events.BlockDiscarded.Trigger(&BlockDiscardedEvent{blockIDFromElementID(droppedBlkID)})
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

func (s *Scheduler) schedule() *Block {
	s.bufferMutex.Lock()
	defer s.bufferMutex.Unlock()
	// Refresh mana cache only at the beginning of the method so that later it's fixed and cannot be
	// updated in the middle of the execution, as it could result in negative deficit values.
	s.AccessManaCache().RefreshCacheIfNecessary()
	s.updateActiveNodesList(s.AccessManaCache().RawAccessManaVector())

	start := s.buffer.Current()
	// no blocks submitted
	if start == nil {
		return nil
	}

	var schedulingNode *schedulerutils.NodeQueue
	rounds := new(big.Rat).SetInt64(math.MaxInt64)
	for q := start; ; {
		blk := q.Front()
		// a block can be scheduled, if it is ready
		// (its issuing time is not in the future and all of its parents are eligible).
		// while loop to skip all the confirmed blocks
		for blk != nil && !clock.SyncedTime().Before(blk.IssuingTime()) {
			var blkID BlockID
			if _, err := blkID.FromBytes(blk.IDBytes()); err != nil {
				panic("BlockID could not be parsed!")
			}
			if s.tangle.ConfirmationOracle.IsBlockConfirmed(blkID) && clock.Since(blk.IssuingTime()) > s.confirmedBlkThreshold {
				// if a block is confirmed, and issued some time ago, don't schedule it and take the next one from the queue
				// do we want to mark those blocks somehow for debugging?
				s.Events.BlockSkipped.Trigger(&BlockSkippedEvent{blkID})
				s.buffer.PopFront()
				blk = q.Front()
			} else {
				// compute how often the deficit needs to be incremented until the block can be scheduled
				remainingDeficit := maxRat(new(big.Rat).Sub(big.NewRat(int64(blk.Size()), 1), s.GetDeficit(q.NodeID())), new(big.Rat))
				// calculate how many rounds we need to skip to accumulate enough deficit.
				// Use for loop to account for float imprecision.
				r := new(big.Rat).Mul(remainingDeficit, new(big.Rat).Inv(s.Quanta(q.NodeID())))
				// find the first node that will be allowed to schedule a block
				if r.Cmp(rounds) < 0 {
					rounds = r
					schedulingNode = q
				}
				break
			}
		}

		q = s.buffer.Next()
		if q == start {
			break
		}
	}

	// if there is no node with a ready block, we cannot schedule anything
	if schedulingNode == nil {
		return nil
	}

	if rounds.Cmp(big.NewRat(0, 1)) > 0 {
		// increment every node's deficit for the required number of rounds
		for q := start; ; {
			s.updateDeficit(q.NodeID(), new(big.Rat).Mul(s.Quanta(q.NodeID()), rounds))

			q = s.buffer.Next()
			if q == start {
				break
			}
		}
	}

	// increment the deficit for all nodes before schedulingNode one more time
	for q := start; q != schedulingNode; q = s.buffer.Next() {
		s.updateDeficit(q.NodeID(), s.Quanta(q.NodeID()))
	}

	// remove the block from the buffer and adjust node's deficit
	blk := s.buffer.PopFront()
	nodeID := identity.NewID(blk.IssuerPublicKey())
	s.updateDeficit(nodeID, new(big.Rat).SetInt64(-int64(blk.Size())))

	return blk.(*Block)
}

func (s *Scheduler) updateActiveNodesList(manaCache map[identity.ID]float64) {
	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	currentNode := s.buffer.Current()
	// use counter to avoid infinite loop in case the start element is removed
	activeNodes := s.buffer.NumActiveNodes()
	// remove nodes that don't have mana and have empty queue
	// this allows nodes with zero mana to issue blocks, however those nodes will only accumulate their deficit
	// when there are blocks in the node's queue
	for i := 0; i < activeNodes; i++ {
		if nodeMana, exists := manaCache[currentNode.NodeID()]; (!exists || nodeMana < MinMana) && currentNode.Size() == 0 {
			s.buffer.RemoveNode(currentNode.NodeID())
			delete(s.deficits, currentNode.NodeID())
			currentNode = s.buffer.Current()
		} else {
			currentNode = s.buffer.Next()
		}
	}

	// update list of active nodes with accumulating deficit
	for nodeID, nodeMana := range manaCache {
		if nodeMana < MinMana {
			continue
		}
		if _, exists := s.deficits[nodeID]; !exists {
			s.deficits[nodeID] = new(big.Rat).SetInt64(0)
			s.buffer.InsertNode(nodeID)
		}
	}
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
			if blk := s.schedule(); blk != nil {
				s.tangle.Storage.BlockMetadata(blk.ID()).Consume(func(blockMetadata *BlockMetadata) {
					if blockMetadata.SetScheduled(true) {
						s.Events.BlockScheduled.Trigger(&BlockScheduledEvent{blk.ID()})
					}
				})
			}

		// on close, exit the loop
		case <-s.shutdownSignal:
			break loop
		}
	}
}

func (s *Scheduler) GetDeficit(nodeID identity.ID) *big.Rat {
	s.deficitsMutex.RLock()
	defer s.deficitsMutex.RUnlock()
	if deficit, exists := s.deficits[nodeID]; !exists {
		return new(big.Rat).SetInt64(0)
	} else {
		return deficit
	}
}

func (s *Scheduler) updateDeficit(nodeID identity.ID, d *big.Rat) {
	deficit := new(big.Rat).Add(s.GetDeficit(nodeID), d)
	if deficit.Sign() < 0 {
		// this will never happen and is just here for debugging purposes
		// TODO: remove print
		panic("scheduler: deficit is less than 0")
	}

	s.deficitsMutex.Lock()
	defer s.deficitsMutex.Unlock()
	s.deficits[nodeID] = minRat(deficit, MaxDeficit)
}

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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func blockIDFromElementID(id schedulerutils.ElementID) (blockID BlockID) {
	_, _ = blockID.FromBytes(id.Bytes())

	return blockID
}
