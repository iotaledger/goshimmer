package database

import (
	"github.com/iotaledger/hive.go/constraints"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/kvstore"
)

type PersistentSlotStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]] struct {
	dbManager *Manager
	realm     kvstore.Realm
}

func NewPersistentSlotStorage[K, V any, KPtr IndexedKey[K], VPtr constraints.MarshalablePtr[V]](dbManager *Manager, realm kvstore.Realm) *PersistentSlotStorage[K, V, KPtr, VPtr] {
	return &PersistentSlotStorage[K, V, KPtr, VPtr]{
		dbManager: dbManager,
		realm:     realm,
	}
}

func (p *PersistentSlotStorage[K, V, KPtr, VPtr]) Get(key K) (value V, exists bool) {
	value, err := kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Get(key)
	return value, err == nil
}

func (p *PersistentSlotStorage[K, V, KPtr, VPtr]) Set(key K, value V) (err error) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Set(key, value)
}

func (p *PersistentSlotStorage[K, V, KPtr, VPtr]) Delete(key K) (success bool) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(KPtr(&key).Index(), p.realm)).Delete(key) == nil
}

func (p *PersistentSlotStorage[K, V, KPtr, VPtr]) Iterate(index slot.Index, callback func(key K, value V) (advance bool), direction ...kvstore.IterDirection) (err error) {
	return kvstore.NewTypedStore[K, V, KPtr, VPtr](p.dbManager.Get(index, p.realm)).Iterate([]byte{}, func(key K, value V) (advance bool) {
		return callback(key, value)
	}, direction...)
}

type IndexedKey[A any] interface {
	slot.IndexedID
	constraints.MarshalablePtr[A]
}
