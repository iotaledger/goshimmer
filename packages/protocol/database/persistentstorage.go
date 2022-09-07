package database

import (
	"github.com/iotaledger/hive.go/core/generics/kvstore"
	. "github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
)

type IndexedKey interface {
	epoch.IndexedID
	kvstore.KeyType
}

type PersistentEpochStorage[K IndexedKey, V any, VPtr kvstore.ValuePtrType[V]] struct {
	dbManager *Manager
	realm     Realm
}

func (p *PersistentEpochStorage[K, V, VPtr]) Get(key K) (value VPtr, exists bool) {
	value, err := kvstore.NewTypedStore[K, V, VPtr](p.dbManager.Get(key.Index(), p.realm)).Get(key)

	return value, err == nil
}
