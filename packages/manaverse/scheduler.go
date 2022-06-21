package manaverse

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/priorityqueue"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Scheduler struct {
	Events *SchedulerEvents

	manaLedger    ManaLedger
	priorityQueue *priorityqueue.PriorityQueue[*Bucket]
	bucketsByMana map[int64]*Bucket
	ticker        *timeutil.PrecisionTicker
	mutex         sync.Mutex

	// internal cache
	unscheduledParentsCounter map[tangle.MessageID]int
	unscheduledBlockChildren  map[tangle.MessageID][]*tangle.Message
}

func NewScheduler(manaLedger ManaLedger) (newScheduler *Scheduler) {
	newScheduler = &Scheduler{
		Events: &SchedulerEvents{
			BlockQueued:    event.New[*tangle.Message](),
			BlockScheduled: event.New[*tangle.Message](),
			BlockDropped:   event.New[*tangle.Message](),
		},

		manaLedger:    manaLedger,
		priorityQueue: priorityqueue.New[*Bucket](),
		bucketsByMana: make(map[int64]*Bucket, 0),

		unscheduledParentsCounter: make(map[tangle.MessageID]int),
		unscheduledBlockChildren:  make(map[tangle.MessageID][]*tangle.Message),
	}
	newScheduler.ticker = timeutil.NewPrecisionTicker(newScheduler.scheduleNextBlock, 500*time.Millisecond)

	return newScheduler
}

func (s *Scheduler) Setup() {
	// TODO: trigger cleanupQueuedBlock on blocks that are confirmed as well
	// TODO: trigger cleanupScheduledBlock on blocks that are confirmed as well
	// s.tangle.Events.BlockConfirmed.Hook(event.NewClosure(s.cleanupQueuedBlock))
	// s.tangle.Events.BlockConfirmed.Hook(event.NewClosure(s.cleanupScheduledBlock))
}

func (s *Scheduler) Push(block *tangle.Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.hasUnscheduledParents(block) {
		s.queueBlock(block)
	}
}

func (s *Scheduler) scheduleNextBlock() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if blockToSchedule := s.nextBlockToSchedule(); blockToSchedule != nil {
		s.cleanupScheduledBlock(blockToSchedule)

		s.Events.BlockScheduled.Trigger(blockToSchedule)
	}
}

func (s *Scheduler) queueBlock(block *tangle.Message) {
	s.cleanupQueuedBlock(block)

	s.bucket(block.BurnedMana()).Push(block)

	s.Events.BlockQueued.Trigger(block)
}

func (s *Scheduler) cleanupQueuedBlock(block *tangle.Message) {
	delete(s.unscheduledParentsCounter, block.ID())
}

func (s *Scheduler) cleanupScheduledBlock(block *tangle.Message) {
	s.decreaseUnscheduledParentsCounters(block.ID())
	delete(s.unscheduledBlockChildren, block.ID())
}

func (s *Scheduler) nextBlockToSchedule() (blockToSchedule *tangle.Message) {
	for blockToSchedule = s.nextBlock(); blockToSchedule != nil && !s.issuerCanIssue(blockToSchedule); blockToSchedule = s.nextBlock() {
		s.Events.BlockDropped.Trigger(blockToSchedule)

		// TODO: REMEMBER IT WAS DROPPED AN IGNORE FUTURE CONE?
	}

	return blockToSchedule
}

func (s *Scheduler) nextBlock() (block *tangle.Message) {
	bucket, success := s.priorityQueue.Peek()
	if !success {
		return nil
	}

	if block, success = bucket.Pop(); !success {
		panic(errors.Errorf("bucket %v should never be empty", bucket))
	}

	if bucket.IsEmpty() {
		s.dropBucket()
	}

	return block
}

func (s *Scheduler) dropBucket() {
	bucket, exists := s.priorityQueue.Pop()
	if !exists {
		panic(errors.New("bucket should never be empty"))
	}

	delete(s.bucketsByMana, bucket.mana)

	// TODO: UPDATE FEE ESTIMATION
}

func (s *Scheduler) issuerCanIssue(block *tangle.Message) (canSchedule bool) {
	return s.manaLedger.DecreaseMana(identity.NewID(block.IssuerPublicKey()), block.BurnedMana()) >= 0
}

func (s *Scheduler) hasUnscheduledParents(block *tangle.Message) (hasUnscheduledParents bool) {
	s.unscheduledBlockChildren[block.ID()] = make([]*tangle.Message, 0)

	if unscheduledParents := s.unscheduledParents(block); unscheduledParents > 0 {
		s.unscheduledParentsCounter[block.ID()] = unscheduledParents

		return true
	}

	return false
}

func (s *Scheduler) unscheduledParents(block *tangle.Message) (unscheduledParents int) {
	for it := lo.Unique(block.Parents()).Iterator(); it.HasNext(); {
		parentID := it.Next()

		if children, isUnscheduled := s.unscheduledBlockChildren[parentID]; isUnscheduled {
			s.unscheduledBlockChildren[parentID] = append(children, block)

			unscheduledParents++
		}
	}

	return unscheduledParents
}

func (s *Scheduler) bucket(mana int64) (bucket *Bucket) {
	bucket, exists := s.bucketsByMana[mana]
	if !exists {
		bucket = NewManaBucket(mana)
		s.bucketsByMana[mana] = bucket
		s.priorityQueue.Push(bucket)
	}

	return bucket
}

func (s *Scheduler) decreaseUnscheduledParentsCounters(blockID tangle.MessageID) {
	for _, child := range s.unscheduledBlockChildren[blockID] {
		if s.unscheduledParentsCounter[child.ID()]--; s.unscheduledParentsCounter[child.ID()] == 0 {
			s.queueBlock(child)
		}
	}
}
