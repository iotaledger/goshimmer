package memstorage

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/shrinkingmap"
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

func (s *Storage[K, V]) Set(key K, value V) (exists bool) {
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
