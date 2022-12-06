package traits

import (
	"sync"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// Committable is a trait that stores information about the latest commitment.
type Committable interface {
	// SetLastCommittedEpoch sets the last committed epoch.
	SetLastCommittedEpoch(index epoch.Index)

	// LastCommittedEpoch returns the last committed epoch.
	LastCommittedEpoch() (index epoch.Index)
}

// NewCommittable creates a new Committable trait.
func NewCommittable() Committable {
	return &committable{}
}

// committable is the implementation of the Committable trait.
type committable struct {
	// lastCommittedEpoch is the last committed epoch.
	lastCommittedEpoch epoch.Index

	// mutex is used to synchronize access to lastCommittedEpoch.
	mutex sync.RWMutex
}

// SetLastCommittedEpoch sets the last committed epoch.
func (c *committable) SetLastCommittedEpoch(index epoch.Index) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.lastCommittedEpoch = index
}

// LastCommittedEpoch returns the last committed epoch.
func (c *committable) LastCommittedEpoch() epoch.Index {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastCommittedEpoch
}
