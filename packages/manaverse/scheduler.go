package manaverse

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/generics/priorityqueue"
	"github.com/iotaledger/hive.go/timeutil"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Scheduler struct {
	priorityQueue *priorityqueue.PriorityQueue[*Bucket]
	bucketsByMana map[uint64]*Bucket
	ticker        *timeutil.PrecisionTicker

	sync.RWMutex
}

func NewScheduler() (newScheduler *Scheduler) {
	newScheduler = &Scheduler{
		priorityQueue: priorityqueue.New[*Bucket](),
		bucketsByMana: make(map[uint64]*Bucket, 0),
	}
	newScheduler.ticker = timeutil.NewPrecisionTicker(newScheduler.scheduleMessages, time.Second)

	return newScheduler
}

func (s *Scheduler) Push(block *tangle.Message) {
	s.Lock()
	defer s.Unlock()

	s.bucket(block.BurnedMana()).Push(block)
}

func (s *Scheduler) bucket(mana uint64) (bucket *Bucket) {
	bucket, exists := s.bucketsByMana[mana]
	if exists {
		return bucket
	}

	bucket = NewManaBucket(mana)
	s.bucketsByMana[mana] = bucket

	return bucket
}

func (s *Scheduler) scheduleMessages() {
	fmt.Println("SEND")
}
