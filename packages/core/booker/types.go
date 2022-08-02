package booker

import (
	"sync"

	"github.com/iotaledger/hive.go/generics/shrinkingmap"
)

type lockableMap[K comparable, V any] struct {
	*shrinkingmap.ShrinkingMap[K, V]

	sync.RWMutex
}

func newLockableMap[K comparable, V any]() (newLockableMap *lockableMap[K, V]) {
	return &lockableMap[K, V]{
		ShrinkingMap: shrinkingmap.New[K, V](),
	}
}
