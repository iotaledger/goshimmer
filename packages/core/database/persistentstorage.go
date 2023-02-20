package database

import (
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/hive.go/core/generics/constraints"
	gkvstore "github.com/iotaledger/hive.go/core/generics/kvstore"
	"github.com/iotaledger/hive.go/core/kvstore"
)

type PersistentEpochStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]] struct {
	dbManager *Manager
	realm     kvstore.Realm
}

func NewPersistentEpochStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]](dbManager *Manager, realm kvstore.Realm) *PersistentEpochStorage[K, V, KPtr, VPtr] {
	return &PersistentEpochStorage[K, V, KPtr, VPtr]{
		dbManager: dbManager,
		realm:     realm,
	}
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Get(key K) (value V, exists bool) {
	value, err := gkvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Get(key)
	return value, err == nil
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Set(key K, value V) (err error) {
	return gkvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Set(key, value)
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Delete(key K) (success bool) {
	return gkvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Delete(key) == nil
}

func (p *PersistentEpochStorage[K, V, KPtr, VPtr]) Iterate(index epoch.Index, callback func(key K, value V) (advance bool), direction ...kvstore.IterDirection) (err error) {
	return gkvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(index, p.realm)).Iterate([]byte{}, func(key K, value V) (advance bool) {
		return callback(key, value)
	}, direction...)
}

type IndexedKey[A any] interface {
	epoch.IndexedID
	constraints.MarshalablePtr[A]
}
