package manaverse

import (
	"github.com/iotaledger/hive.go/generics/priorityqueue"

	"github.com/iotaledger/goshimmer/packages/tangle"
)

type Bucket struct {
	mana int64

	*priorityqueue.PriorityQueue[*tangle.Message]
}

func newManaBucket(mana int64) *Bucket {
	return &Bucket{
		mana:          mana,
		PriorityQueue: priorityqueue.New[*tangle.Message](),
	}
}

func (b *Bucket) Compare(other *Bucket) int {
	switch true {
	case b.mana < other.mana:
		return -1
	case b.mana > other.mana:
		return 1
	default:
		return 0
	}
}
