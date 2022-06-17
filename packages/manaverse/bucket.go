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

func (b *Bucket) Mana() (mana uint64) {
	return b.mana
}

func (b *Bucket) Push(block *tangle.Message) {
	b.Lock()
	defer b.Unlock()

	b.priorityQueue.Push(block)
}

func (b *Bucket) Compare(other *Bucket) int {
	if b.Mana() < other.Mana() {
		return -1
	}

	if b.Mana() > other.Mana() {
		return 1
	}

	return 0
}
