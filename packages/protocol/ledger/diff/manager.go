package diff

import "github.com/iotaledger/hive.go/core/kvstore"

type Manager struct{
	store *kvstore.KVStore
}

func NewManager(store *kvstore.KVStore) *Manager {
	return &Manager{
		store: store,
	}
}


