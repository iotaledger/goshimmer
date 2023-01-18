package memstorage

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

// EpochStorage is an evictable storage that stores storages for epochs.
type EpochStorage[K comparable, V any] struct {
	cache *shrinkingmap.ShrinkingMap[epoch.Index, *Storage[K, V]]
	mutex sync.Mutex
}

// NewEpochStorage creates a new epoch storage.
func NewEpochStorage[K comparable, V any]() *EpochStorage[K, V] {
	return &EpochStorage[K, V]{
		cache: shrinkingmap.New[epoch.Index, *Storage[K, V]](),
	}
}

// Evict evicts the storage for the given index.
func (e *EpochStorage[K, V]) Evict(index epoch.Index) (evictedStorage *Storage[K, V]) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if storage, exists := e.cache.Get(index); exists {
		evictedStorage = storage

		e.cache.Delete(index)
	}

	return
}

// Get returns the storage for the given index.
func (e *EpochStorage[K, V]) Get(index epoch.Index, createIfMissing ...bool) (storage *Storage[K, V]) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

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

// ForEach iterates over all storages.
func (e *EpochStorage[K, V]) ForEach(f func(index epoch.Index, storage *Storage[K, V])) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	e.cache.ForEach(func(index epoch.Index, storage *Storage[K, V]) bool {
		f(index, storage)

		return true
	})
}
