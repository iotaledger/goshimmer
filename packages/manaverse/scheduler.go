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

	confirmationOracle tangle.ConfirmationOracle
	manaLedger         ManaLedger
	priorityQueue      *priorityqueue.PriorityQueue[*Bucket]
	bucketsByMana      map[int64]*Bucket
	currentBucket      int64
	ticker             *timeutil.PrecisionTicker
	mutex              sync.Mutex

	// unqueuedBlocks
	unqueuedBlocks    map[tangle.MessageID]int
	unscheduledBlocks map[tangle.MessageID][]*tangle.Message
}

func NewScheduler(confirmationOracle tangle.ConfirmationOracle, manaLedger ManaLedger) (newScheduler *Scheduler) {
	newScheduler = &Scheduler{
		Events:             newSchedulerEvents(),
		confirmationOracle: confirmationOracle,
		manaLedger:         manaLedger,
		priorityQueue:      priorityqueue.New[*Bucket](),
		bucketsByMana:      make(map[int64]*Bucket, 0),
		currentBucket:      -1,
		unqueuedBlocks:     make(map[tangle.MessageID]int),
		unscheduledBlocks:  make(map[tangle.MessageID][]*tangle.Message),
	}
	newScheduler.ticker = timeutil.NewPrecisionTicker(newScheduler.ScheduleBlock, 500*time.Millisecond)

	return newScheduler
}

func (s *Scheduler) Setup() {
	s.confirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(s.onMessageConfirmed))
}

func (s *Scheduler) onMessageConfirmed(event *tangle.MessageConfirmedEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.markBlockQueued(event.Message)
	s.markBlockScheduled(event.Message, time.Now())
}

func (s *Scheduler) Push(block *tangle.Message) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !s.hasUnscheduledParents(block) {
		s.queueBlock(block, time.Now())
	}
}

func (s *Scheduler) ScheduleBlock() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.scheduleNextBlock(time.Now())
}

func (s *Scheduler) queueBlock(block *tangle.Message, now time.Time) {
	bucket := s.bucket(block.BurnedMana())
	bucket.Push(block)

	s.markBlockQueued(block)

	s.Events.BlockQueued.Trigger(newSchedulerBlockEvent(block, bucket.mana, now))
}

func (s *Scheduler) markBlockQueued(block *tangle.Message) {
	delete(s.unqueuedBlocks, block.ID())
}

func (s *Scheduler) markBlockScheduled(block *tangle.Message, now time.Time) {
	s.decreaseUnscheduledParentCountersOfChildren(block.ID(), now)

	delete(s.unscheduledBlocks, block.ID())
}

func (s *Scheduler) dropBucket() {
	bucket, exists := s.priorityQueue.Pop()
	if !exists {
		panic(errors.New("bucket should never be empty"))
	}

	delete(s.bucketsByMana, bucket.mana)
	s.currentBucket = -1
}

func (s *Scheduler) scheduleNextBlock(now time.Time) {
	blockToSchedule, bucket := s.nextBlock(now)
	for ; blockToSchedule != nil && !s.issuerHasEnoughMana(blockToSchedule); blockToSchedule, bucket = s.nextBlock(now) {
		s.Events.BlockDropped.Trigger(newSchedulerBlockEvent(blockToSchedule, bucket, now))

	}

	if blockToSchedule == nil {
		return
	}

	s.markBlockScheduled(blockToSchedule, now)

	s.Events.BlockScheduled.Trigger(newSchedulerBlockEvent(blockToSchedule, bucket, now))
}

func (s *Scheduler) nextBlock(now time.Time) (block *tangle.Message, bucket int64) {
	firstBucket, success := s.priorityQueue.Peek()
	if !success {
		return nil, 0
	}

	if s.currentBucket != firstBucket.mana {
		s.currentBucket = firstBucket.mana
		s.Events.BucketProcessingStarted.Trigger(newSchedulerBucketEvent(firstBucket.mana, now))
	}

	if block, success = firstBucket.Pop(); !success {
		panic(errors.Errorf("bucket %v should never be empty", firstBucket))
	}

	if firstBucket.IsEmpty() {
		s.Events.BucketProcessingFinished.Trigger(newSchedulerBucketEvent(firstBucket.mana, now))

		s.dropBucket()
	}

	return block, firstBucket.mana
}

func (s *Scheduler) issuerHasEnoughMana(block *tangle.Message) (canSchedule bool) {
	return s.manaLedger.DecreaseMana(identity.NewID(block.IssuerPublicKey()), block.BurnedMana()) >= 0
}

func (s *Scheduler) hasUnscheduledParents(block *tangle.Message) (hasUnscheduledParents bool) {
	s.unscheduledBlocks[block.ID()] = make([]*tangle.Message, 0)

	if unscheduledParents := s.unscheduledParents(block); unscheduledParents > 0 {
		s.unqueuedBlocks[block.ID()] = unscheduledParents

		return true
	}

	return false
}

func (s *Scheduler) unscheduledParents(block *tangle.Message) (unscheduledParents int) {
	for it := lo.Unique(block.Parents()).Iterator(); it.HasNext(); {
		parentID := it.Next()

		if children, isUnscheduled := s.unscheduledBlocks[parentID]; isUnscheduled {
			s.unscheduledBlocks[parentID] = append(children, block)

			unscheduledParents++
		}
	}

	return unscheduledParents
}

func (s *Scheduler) bucket(mana int64) (bucket *Bucket) {
	bucket, exists := s.bucketsByMana[mana]
	if !exists {
		bucket = newManaBucket(mana)
		s.bucketsByMana[mana] = bucket
		s.priorityQueue.Push(bucket)
	}

	return bucket
}

func (s *Scheduler) decreaseUnscheduledParentCountersOfChildren(blockID tangle.MessageID, now time.Time) {
	for _, child := range s.unscheduledBlocks[blockID] {
		if s.unqueuedBlocks[child.ID()]--; s.unqueuedBlocks[child.ID()] == 0 {
			s.queueBlock(child, now)
		}
	}
}
