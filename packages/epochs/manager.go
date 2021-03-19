package epochs

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/objectstorage"
)

const (
	// defaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2021-03-19 9:00:00 UTC.
	defaultGenesisTime int64 = 1616144400
	// defaultInterval is the interval of epochs, i.e., their duration, and is 30 minutes (specified in seconds).
	defaultInterval = int64(30 * 60)

	defaultOracleEpochShift = 2
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
	options      *ManagerOptions
	epochStorage *objectstorage.ObjectStorage
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		Store:       mapdb.NewMapDB(),
		GenesisTime: defaultGenesisTime,
		Interval:    defaultInterval,
	}

	for _, option := range opts {
		option(options)
	}

	osFactory := objectstorage.NewFactory(options.Store, database.PrefixEpochs)
	return &Manager{
		epochStorage: osFactory.New(PrefixEpochStorage, EpochFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
	}
}

func (m *Manager) TimeToEpochID(t time.Time) (epochID ID) {
	elapsedSeconds := t.Unix() - m.options.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return ID(elapsedSeconds / m.options.Interval)
}

func (m *Manager) TimeToOracleEpochID(t time.Time) (oracleEpochID ID) {
	epochID := m.TimeToEpochID(t)
	// TODO: introduce parameter for oracle epoch shift
	if epochID < 2 {
		return 0
	}

	return epochID - 2
}

func (m *Manager) Update(t time.Time, nodeID identity.ID) {
	epochID := m.TimeToEpochID(t)

	// create typed version of the stored Epoch
	(&CachedEpoch{CachedObject: m.epochStorage.ComputeIfAbsent(epochID.Bytes(), func(key []byte) objectstorage.StorableObject {
		newEpoch := NewEpoch(epochID)
		newEpoch.SetModified()
		newEpoch.Persist()

		return newEpoch
	})}).Consume(func(epoch *Epoch) {
		// add active node to the corresponding epoch
		epoch.AddNode(nodeID)
	})
}

func (m *Manager) ActiveMana(t time.Time) (mana map[identity.ID]float64) {
	epochID := m.TimeToEpochID(t)

	if !(&CachedEpoch{CachedObject: m.epochStorage.Load(epochID.Bytes())}).Consume(func(epoch *Epoch) {
		if !epoch.ManaRetrieved() {
			// TODO: request from mana plugin if non existent
		}

		mana = epoch.Mana()
	}) {
		panic(fmt.Sprintf("%s does not exist", epochID))
	}

	return
}

// Shutdown shuts down the Manager and causes its content to be persisted to the disk.
func (m *Manager) Shutdown() {
	m.epochStorage.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ManagerOptions ///////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of optional parameters that can be handed into the constructor of the
// Manager to configure its behavior.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container for all configurable parameters of the Manager.
type ManagerOptions struct {
	Store       kvstore.KVStore
	GenesisTime int64
	Interval    int64
}

// Store is a ManagerOption that allows to specify which storage layer is supposed to be used to persist data.
func Store(store kvstore.KVStore) ManagerOption {
	return func(options *ManagerOptions) {
		options.Store = store
	}
}

// GenesisTime is a ManagerOption that allows to define the time of the genesis, i.e., the start of the epochs,
// specified in Unix time (seconds).
func GenesisTime(genesisTime int64) ManagerOption {
	return func(options *ManagerOptions) {
		options.GenesisTime = genesisTime
	}
}

// Interval is a ManagerOption that allows to define the epoch interval, i.e., the duration of each Epoch, specified
// in seconds.
func Interval(interval int64) ManagerOption {
	return func(options *ManagerOptions) {
		options.Interval = interval
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
