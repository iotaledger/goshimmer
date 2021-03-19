package epochs

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// 2020-03-19 9:00:00 UTC in seconds
	genesisTime int64 = 1616144400
	// 30 minutes
	interval = int64(30 * 60)
)

// region Object Storage Parameters ////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixEpochStorage defines the storage prefix for the epoch storage.
	PrefixEpochStorage byte = iota

	// cacheTime defines the duration that the object storage caches objects.
	cacheTime = 60 * time.Second
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the managing entity for the Epoch related business logic. It is stateful and automatically stores its
// state in an underlying KVStore.
type Manager struct {
	epochStorage *objectstorage.ObjectStorage
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(store kvstore.KVStore) *Manager {
	osFactory := objectstorage.NewFactory(store, database.PrefixEpochs)

	return &Manager{
		epochStorage: osFactory.New(PrefixEpochStorage, EpochFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
	}
}

func (m *Manager) Update(t time.Time, nodeID identity.ID) {
	epochID := timeToEpochID(t)

	cachedObject := m.epochStorage.ComputeIfAbsent(epochID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newEpoch := NewEpoch(epochID)
		newEpoch.SetModified()
		newEpoch.Persist()

		return newEpoch
	})

	// create typed version of the stored Epoch
	cachedEpoch := &CachedEpoch{CachedObject: cachedObject}
	defer cachedEpoch.Release()

	// add active node to the corresponding epoch
	cachedEpoch.Consume(func(epoch *Epoch) {
		epoch.AddNode(nodeID)
	})
}

// Shutdown shuts down the Manager and causes its content to be persisted to the disk.
func (m *Manager) Shutdown() {
	m.epochStorage.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

func timeToEpochID(t time.Time) (epochID ID) {
	elapsedSeconds := t.Unix() - genesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return ID(elapsedSeconds / interval)
}
