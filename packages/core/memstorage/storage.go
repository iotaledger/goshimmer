package memstorage

import (
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
)

type Storage[K comparable, V any] struct {
	storage *shrinkingmap.ShrinkingMap[K, V]
	sync.RWMutex
}

func New[K comparable, V any]() *Storage[K, V] {
	return &Storage[K, V]{
		storage: shrinkingmap.New[K, V](),
	}
}

func (s *Storage[K, V]) Get(key K) (value V, exists bool) {
	s.RLock()
	defer s.RUnlock()

	return s.storage.Get(key)
}

func (s *Storage[K, V]) Has(key K) (has bool) {
	s.RLock()
	defer s.RUnlock()

	return lo.Return2(s.storage.Get(key))
}

func (s *Storage[K, V]) Delete(key K) (deleted bool) {
	s.Lock()
	defer s.Unlock()

	return s.storage.Delete(key)
}

func (s *Storage[K, V]) RetrieveOrCreate(key K, defaultValueFunc func() V) (value V, created bool) {
	s.Lock()
	defer s.Unlock()

	if existingValue, exists := s.storage.Get(key); exists {
		return existingValue, false
	}

	value = defaultValueFunc()
	s.storage.Set(key, value)

	return value, true
}

// ForEachKey iterates through the map and calls the consumer for every element.
// Returning false from this function indicates to abort the iteration.
func (s *Storage[K, V]) ForEachKey(callback func(K) bool) {
	s.RLock()
	defer s.RUnlock()

	s.storage.ForEachKey(callback)
}

// ForEach iterates through the map and calls the consumer for every element.
// Returning false from this function indicates to abort the iteration.
func (s *Storage[K, V]) ForEach(callback func(K, V) bool) {
	s.RLock()
	defer s.RUnlock()

	s.storage.ForEach(callback)
}

func (s *Storage[K, V]) First() (key K, value V) {
	s.RLock()
	defer s.RUnlock()

	s.storage.ForEach(func(k K, v V) bool {
		key = k
		value = v

		return false
	})

	return
}

func (s *Storage[K, V]) Set(key K, value V) (updated bool) {
	s.Lock()
	defer s.Unlock()

	return s.storage.Set(key, value)
}

func (s *Storage[K, V]) ExecuteIfAbsent(key K, callback func()) {
	s.RLock()
	defer s.RUnlock()

	if _, exists := s.storage.Get(key); exists {
		return
	}

	callback()
}

func (s *Storage[K, V]) StoreIfAbsent(key K, value V) (stored bool) {
	s.Lock()
	defer s.Unlock()

	if _, exists := s.storage.Get(key); exists {
		return false
	}

	s.storage.Set(key, value)

	return true
}

func (s *Storage[K, V]) Size() (size int) {
	s.RLock()
	defer s.RUnlock()

	return s.storage.Size()
}

func (s *Storage[K, V]) IsEmpty() (isEmpty bool) {
	return s.Size() == 0
}

func (s *Storage[K, V]) AsMap() map[K]V {
	s.RLock()
	defer s.RUnlock()

	return s.storage.AsMap()
}
