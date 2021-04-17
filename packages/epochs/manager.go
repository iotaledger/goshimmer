package epochs

import (
	"math/rand"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/database"
)

const (
	// DefaultGenesisTime is the default time (Unix in seconds) of the genesis, i.e., the start of the epochs at 2021-03-19 9:00:00 UTC.
	DefaultGenesisTime int64 = 1616144400

	// defaultInterval is the default interval of epochs, i.e., their duration, and is 30 minutes (specified in seconds).
	defaultInterval int64 = 30 * 60

	// defaultOracleEpochShift is the default shift of the oracle epoch. E.g., current epoch=4 -> oracle epoch=2
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

// ManaRetrieverFunc is a function type to retrieve consensus mana (e.g. via the mana plugin)
type ManaRetrieverFunc func(t time.Time) map[identity.ID]float64

// Manager is the managing entity for the Epoch related business logic. It is stateful and automatically stores its
// state in an underlying KVStore.
type Manager struct {
	options      *ManagerOptions
	epochStorage *objectstorage.ObjectStorage
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(opts ...ManagerOption) *Manager {
	options := &ManagerOptions{
		Store:            mapdb.NewMapDB(),
		CacheTime:        cacheTime,
		GenesisTime:      DefaultGenesisTime,
		Interval:         defaultInterval,
		OracleEpochShift: defaultOracleEpochShift,
	}

	for _, option := range opts {
		option(options)
	}

	if options.ManaRetriever == nil {
		panic("the option ManaRetriever must be defined so that ActiveMana for an epoch can be determined")
	}

	osFactory := objectstorage.NewFactory(options.Store, database.PrefixEpochs)
	return &Manager{
		options:      options,
		epochStorage: osFactory.New(PrefixEpochStorage, EpochFromObjectStorage, objectstorage.CacheTime(options.CacheTime), objectstorage.LeakDetectionEnabled(false)),
	}
}

// TimeToEpochID calculates the epoch ID for the given time.
func (m *Manager) TimeToEpochID(t time.Time) (epochID ID) {
	elapsedSeconds := t.Unix() - m.options.GenesisTime
	if elapsedSeconds <= 0 {
		return 0
	}

	return ID(elapsedSeconds / m.options.Interval)
}

// TimeToOracleEpochID calculates the oracle epoch ID for the given time.
func (m *Manager) TimeToOracleEpochID(t time.Time) (oracleEpochID ID) {
	epochID := m.TimeToEpochID(t)
	oracleEpochID = epochID - m.options.OracleEpochShift

	// default to epoch 0 if oracle epoch < shift as it is not defined
	if epochID < m.options.OracleEpochShift || oracleEpochID < m.options.OracleEpochShift {
		return 0
	}

	return
}

// EpochIDToStartTime calculates the start time of the given epoch.
func (m *Manager) EpochIDToStartTime(epochID ID) time.Time {
	startUnix := m.options.GenesisTime + int64(epochID)*m.options.Interval
	return time.Unix(startUnix, 0)
}

// EpochIDToEndTime calculates the end time of the given epoch.
func (m *Manager) EpochIDToEndTime(epochID ID) time.Time {
	endUnix := m.options.GenesisTime + int64(epochID)*m.options.Interval + m.options.Interval - 1
	return time.Unix(endUnix, 0)
}

// Update marks the given nodeID as active during the corresponding epoch (defined by the time t).
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

// RelativeNodeMana returns the active consensus mana of the nodeID that is valid for the given time t as well as the epoch ID.
func (m *Manager) RelativeNodeMana(nodeID identity.ID, t time.Time) (ownWeight, totalWeight float64, epochID ID) {
	epochID = m.TimeToOracleEpochID(t)
	manaPerID, totalMana := m.ActiveMana(epochID)

	return manaPerID[nodeID], totalMana, epochID
}

// ActiveMana returns the active consensus mana that is valid for the given time t. Active consensus mana is always
// retrieved from the oracle epoch.
func (m *Manager) ActiveMana(epochID ID) (manaPerID map[identity.ID]float64, totalMana float64) {
	if !m.Epoch(epochID).Consume(func(epoch *Epoch) {
		if !epoch.ManaRetrieved() {
			consensusMana := m.options.ManaRetriever(m.EpochIDToEndTime(epochID))
			epoch.SetMana(consensusMana)
		}

		manaPerID = epoch.Mana()
		totalMana = epoch.TotalMana()
	}) {
		// TODO: improve this: always default to epoch 0
		epochID = ID(0)
		(&CachedEpoch{CachedObject: m.epochStorage.ComputeIfAbsent(epochID.Bytes(), func(key []byte) objectstorage.StorableObject {
			newEpoch := NewEpoch(epochID)
			consensusMana := m.options.ManaRetriever(m.EpochIDToEndTime(epochID))
			newEpoch.SetMana(consensusMana, true)

			newEpoch.SetModified()
			newEpoch.Persist()

			return newEpoch
		})}).Consume(func(epoch *Epoch) {
			manaPerID = epoch.Mana()
			totalMana = epoch.TotalMana()
		})
	}

	return
}

// Epoch retrieves an Epoch from the epoch storage.
func (m *Manager) Epoch(epochID ID) *CachedEpoch {
	return &CachedEpoch{CachedObject: m.epochStorage.Load(epochID.Bytes())}
}

// Shutdown shuts down the Manager and causes its content to be persisted to the disk.
func (m *Manager) Shutdown() {
	m.epochStorage.Shutdown()
}

// randomTimeInEpoch is a utility function useful for testing to generate a random time within an epoch.
func (m *Manager) randomTimeInEpoch(epochID ID) time.Time {
	startUnix := m.options.GenesisTime + int64(epochID)*m.options.Interval
	return time.Unix(startUnix+rand.Int63n(m.options.Interval), 0)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ManagerOptions ///////////////////////////////////////////////////////////////////////////////////////////////

// ManagerOption represents the return type of optional parameters that can be handed into the constructor of the
// Manager to configure its behavior.
type ManagerOption func(options *ManagerOptions)

// ManagerOptions is a container for all configurable parameters of the Manager.
type ManagerOptions struct {
	Store            kvstore.KVStore
	CacheTime        time.Duration
	GenesisTime      int64
	Interval         int64
	OracleEpochShift ID
	ManaRetriever    ManaRetrieverFunc
}

// Store is a ManagerOption that allows to specify which storage layer is supposed to be used to persist data.
func Store(store kvstore.KVStore) ManagerOption {
	return func(options *ManagerOptions) {
		options.Store = store
	}
}

// CacheTime is a ManagerOption that allows to define the cache time of the underlying object storage.
func CacheTime(cacheTime time.Duration) ManagerOption {
	return func(options *ManagerOptions) {
		options.CacheTime = cacheTime
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

// OracleEpochShift is a ManagerOption that allows to define the shift of the oracle epoch.
func OracleEpochShift(shift int) ManagerOption {
	return func(options *ManagerOptions) {
		options.OracleEpochShift = ID(shift)
	}
}

// ManaRetriever is a ManagerOption that allows to define mana retriever function for the epochs.
func ManaRetriever(manaRetriever ManaRetrieverFunc) ManagerOption {
	return func(options *ManagerOptions) {
		options.ManaRetriever = manaRetriever
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
