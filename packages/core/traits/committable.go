package traits

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/kvstore"

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
func NewCommittable(store kvstore.KVStore, keyBytes ...byte) (newCommittable Committable) {
	if len(keyBytes) == 0 {
		panic("keyBytes must not be empty")
	}

	return (&committable{
		store:    store,
		keyBytes: keyBytes,
	}).init()
}

// committable is the implementation of the Committable trait.
type committable struct {
	store kvstore.KVStore

	keyBytes []byte

	// lastCommittedEpoch is the last committed epoch.
	lastCommittedEpoch epoch.Index

	// mutex is used to synchronize access to lastCommittedEpoch.
	mutex sync.RWMutex
}

// SetLastCommittedEpoch sets the last committed epoch.
func (c *committable) SetLastCommittedEpoch(index epoch.Index) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lastCommittedEpoch != index {
		c.lastCommittedEpoch = index

		if err := c.storeLastCommittedEpoch(index); err != nil {
			panic(err)
		}
	}
}

// LastCommittedEpoch returns the last committed epoch.
func (c *committable) LastCommittedEpoch() epoch.Index {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	return c.lastCommittedEpoch
}

// init initializes the committable trait.
func (c *committable) init() (self *committable) {
	lastCommittedEpoch, err := c.loadLastCommittedEpoch()
	if err != nil {
		panic(err)
	}

	c.lastCommittedEpoch = lastCommittedEpoch

	return c
}

func (c *committable) loadLastCommittedEpoch() (lastCommittedEpoch epoch.Index, err error) {
	lastCommittedEpochBytes, err := c.store.Get(c.keyBytes)
	if err != nil {
		return 0, lo.Cond(errors.Is(err, kvstore.ErrKeyNotFound), nil, errors.Errorf("failed to load last committed epoch: %w", err))
	}

	if lastCommittedEpoch, _, err = epoch.IndexFromBytes(lastCommittedEpochBytes); err != nil {
		return 0, errors.Errorf("failed to parse last committed epoch: %w", err)
	}

	return
}

func (c *committable) storeLastCommittedEpoch(index epoch.Index) (err error) {
	if err = c.store.Set(c.keyBytes, index.Bytes()); err != nil {
		return errors.Errorf("failed to store weight of attestations for epoch %d: %w", index, err)
	}

	return
}
