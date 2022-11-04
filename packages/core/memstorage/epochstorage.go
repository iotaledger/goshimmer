package memstorage

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type EpochStorage[K comparable, V any] struct {
	cache *shrinkingmap.ShrinkingMap[epoch.Index, *Storage[K, V]]

	sync.RWMutex
}

func NewEpochStorage[K comparable, V any]() *EpochStorage[K, V] {
	return &EpochStorage[K, V]{
		cache: shrinkingmap.New[epoch.Index, *Storage[K, V]](),
	}
}

func (e *EpochStorage[K, V]) EvictEpoch(index epoch.Index) {
	e.Lock()
	defer e.Unlock()

	e.cache.Delete(index)
}

func (e *EpochStorage[K, V]) Get(index epoch.Index, createIfMissing ...bool) (storage *Storage[K, V]) {
	e.Lock()
	defer e.Unlock()

	storage, exists := e.cache.Get(index)
	if exists {
		return storage
	}

	if len(createIfMissing) == 0 || !createIfMissing[0] {
		return nil
	}

	storage = New[K, V]()
	e.cache.Set(index, storage)

	return storage

}
