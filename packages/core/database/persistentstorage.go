package database

import (
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/kvstore"
	. "github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type PersistentEpochStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]] struct {
	dbManager *Manager
	realm     Realm
}

func NewPersistentEpochStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]](dbManager *Manager, realm Realm) *PersistentEpochStorage[K, V, KPtr, VPtr] {
	return &PersistentEpochStorage[K, V, KPtr, VPtr]{
		dbManager: dbManager,
		realm:     realm,
	}
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Get(key K) (value VPtr, exists bool) {
	value, err := kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get((KPtr)(&key).Index(), p.realm)).Get(key)
	return value, err == nil
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Set(key K, value VPtr) (success bool) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get((KPtr)(&key).Index(), p.realm)).Set(key, value) == nil
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Delete(key K) (success bool) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get((KPtr)(&key).Index(), p.realm)).Delete(key) == nil
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Iterate(index epoch.Index, callback func(key K, value VPtr) (advance bool), direction ...IterDirection) (err error) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(index, p.realm)).Iterate([]byte{}, func(key K, value VPtr) (advance bool) {
		return callback(key, value)
	}, direction...)
}

type IndexedKey[A any] interface {
	epoch.IndexedID
	constraints.MarshalablePtr[A]
}
