package manaverse

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/priorityqueue"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Bucket struct {
	mana          uint64
	priorityQueue *priorityqueue.PriorityQueue[*tangle.Message]

	sync.RWMutex
}

func NewManaBucket(mana uint64) *Bucket {
	return &Bucket{
		mana:          mana,
		priorityQueue: priorityqueue.New[*tangle.Message](),
	}
}

func (f *Bucket) Mana() (mana uint64) {
	return f.mana
}

func (f *Bucket) Push(block *tangle.Message) {
	f.Lock()
	defer f.Unlock()

	f.priorityQueue.Push(block)
}

func (f *Bucket) Compare(other *Bucket) int {
	if f.Mana() < other.Mana() {
		return -1
	}

	if f.Mana() > other.Mana() {
		return 1
	}

	return 0
}
