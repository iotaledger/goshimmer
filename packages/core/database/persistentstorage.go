package database

import (
	"github.com/iotaledger/hive.go/core/generics/constraints"
	"github.com/iotaledger/hive.go/core/generics/kvstore"
	. "github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type PersistentEpochStorage[K IndexedKey, V constraints.Marshalable] struct {
	dbManager *Manager
	realm     Realm
}

func NewPersistentEpochStorage[K IndexedKey, V constraints.Marshalable](dbManager *Manager, realm Realm) *PersistentEpochStorage[K, V] {
	return &PersistentEpochStorage[K, V]{
		dbManager: dbManager,
		realm:     realm,
	}
}

func (p *PersistentEpochStorage[K, V]) Get(key K) (value *V, exists bool) {
	value, err := kvstore.NewTypedStore[K, V](p.dbManager.Get(key.Index(), p.realm)).Get(key)
	return value, err == nil
}

func (p *PersistentEpochStorage[K, V]) Set(key K, value *V) (success bool) {
	return kvstore.NewTypedStore[K, V](p.dbManager.Get(key.Index(), p.realm)).Set(key, value) == nil
}

func (p *PersistentEpochStorage[K, V]) Delete(key K) (success bool) {
	return kvstore.NewTypedStore[K, V](p.dbManager.Get(key.Index(), p.realm)).Set(key, nil) == nil
}

func (p *PersistentEpochStorage[K, V]) Iterate(index epoch.Index, callback func(key K, value *V) (advance bool), direction ...IterDirection) (err error) {
	return kvstore.NewTypedStore[K, V](p.dbManager.Get(index, p.realm)).Iterate([]byte{}, func(key K, value *V) (advance bool) {
		return callback(key, value)
	}, direction...)
}

type IndexedKey interface {
	epoch.IndexedID
	constraints.Serializable
}

type IndexedMarshalable[K any] interface {
	epoch.IndexedID
	constraints.MarshalablePtr[K]
}
