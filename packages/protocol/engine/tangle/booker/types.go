package booker

import (
	"sync"

	"github.com/iotaledger/hive.go/lo"
)

// region LockableSlice ////////////////////////////////////////////////////////////////////////////////////////////////

type LockableSlice[T any] struct {
	slice []T
	mutex sync.RWMutex
}

func NewLockableSlice[T any]() (newLockableSlice *LockableSlice[T]) {
	return &LockableSlice[T]{
		slice: make([]T, 0),
	}
}

func (l *LockableSlice[T]) Append(newElement ...T) {
	l.mutex.Lock()
	defer l.mutex.Unlock()

	l.slice = append(l.slice, newElement...)
}

func (l *LockableSlice[T]) Slice() (slice []T) {
	l.mutex.RLock()
	defer l.mutex.RUnlock()

	return lo.CopySlice(l.slice)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region LockableMap //////////////////////////////////////////////////////////////////////////////////////////////////

type LockableMap[K comparable, V any] struct {
	m     map[K]V
	mutex sync.RWMutex
}

func NewLockableMap[K comparable, V any]() (newLockableMap *LockableMap[K, V]) {
	return &LockableMap[K, V]{
		m: make(map[K]V),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
