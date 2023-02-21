package traits

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/storage/typedkey"
	"github.com/iotaledger/hive.go/kvstore"
)

// Committable is a trait that stores information about the latest commitment.
type Committable interface {
	// SetLastCommittedEpoch sets the last committed epoch.
	SetLastCommittedEpoch(index epoch.Index)

	// LastCommittedEpoch returns the last committed epoch.
	LastCommittedEpoch() (index epoch.Index)
}

// NewCommittable creates a new Committable trait.
func NewCommittable(store kvstore.KVStore, keyBytes ...byte) (newCommittable Committable) {
	return &committable{
		lastCommittedEpoch: typedkey.NewGenericType[epoch.Index](store, keyBytes...),
	}
}

// committable is the implementation of the Committable trait.
type committable struct {
	lastCommittedEpoch *typedkey.GenericType[epoch.Index]
}

// SetLastCommittedEpoch sets the last committed epoch.
func (c *committable) SetLastCommittedEpoch(index epoch.Index) {
	if c.lastCommittedEpoch.Get() != index {
		c.lastCommittedEpoch.Set(index)
	}
}

// LastCommittedEpoch returns the last committed epoch.
func (c *committable) LastCommittedEpoch() epoch.Index {
	return c.lastCommittedEpoch.Get()
}
