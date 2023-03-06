package traits

import (
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/typedkey"
)

// Committable is a trait that stores information about the latest commitment.
type Committable interface {
	// SetLastCommittedSlot sets the last committed slot.
	SetLastCommittedSlot(index slot.Index)

	// LastCommittedSlot returns the last committed slot.
	LastCommittedSlot() (index slot.Index)
}

// NewCommittable creates a new Committable trait.
func NewCommittable(store kvstore.KVStore, keyBytes ...byte) (newCommittable Committable) {
	return &committable{
		lastCommittedSlot: typedkey.NewGenericType[slot.Index](store, keyBytes...),
	}
}

// committable is the implementation of the Committable trait.
type committable struct {
	lastCommittedSlot *typedkey.GenericType[slot.Index]
}

// SetLastCommittedSlot sets the last committed slot.
func (c *committable) SetLastCommittedSlot(index slot.Index) {
	if c.lastCommittedSlot.Get() != index {
		c.lastCommittedSlot.Set(index)
	}
}

// LastCommittedSlot returns the last committed slot.
func (c *committable) LastCommittedSlot() slot.Index {
	return c.lastCommittedSlot.Get()
}
