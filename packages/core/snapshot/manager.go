package snapshot

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/kvstore"
)

const (
	prefixSolidEntryPoint byte = iota
)

type Manager struct {
	tangle *tangleold.Tangle

	baseStore        kvstore.KVStore
	solidEntryPoints map[epoch.Index]*objectstorage.ObjectStorage[*tangleold.Block]

	snapshotOptions *options
	shutdownOnce    sync.Once
}

func NewManager(store kvstore.KVStore, t *tangleold.Tangle, opts ...Option) (new *Manager) {
	new = &Manager{
		tangle:          t,
		snapshotOptions: newOptions(opts...),
	}

	new.baseStore = store
	new.solidEntryPoints = make(map[epoch.Index]*objectstorage.ObjectStorage[*tangleold.Block])

	new.tangle.Storage.Events.BlockStored.Attach(event.NewClosure(func(e *tangleold.BlockStoredEvent) {
		e.Block.ForEachParent(func(parent tangleold.Parent) {
			index := parent.ID.EpochIndex
			if index < e.Block.EI() && index > e.Block.LatestConfirmedEpoch() {
				new.insertSolidEntryPoint(parent.ID)
			}
		})
	}))

	new.tangle.ConfirmationOracle.Events().BlockOrphaned.Attach(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		new.removeSolidEntryPoint(event.Block, event.Block.LatestConfirmedEpoch())
	}))

	return
}

func (m *Manager) insertSolidEntryPoint(id tangleold.BlockID) {
	m.tangle.Storage.Block(id).Consume(func(b *tangleold.Block) {
		s, ok := m.solidEntryPoints[b.EI()]
		if !ok {
			s = objectstorage.NewStructStorage[tangleold.Block](
				objectstorage.NewStoreWithRealm(m.baseStore, database.PrefixSnapshot, prefixSolidEntryPoint),
				m.snapshotOptions.cacheTimeProvider.CacheTime(m.snapshotOptions.solidEntryPointCacheTime),
				objectstorage.LeakDetectionEnabled(false),
				objectstorage.StoreOnCreation(true),
			)
			m.solidEntryPoints[b.EI()] = s
		}
		s.Store(b).Release()
	})
}

func (m *Manager) removeSolidEntryPoint(b *tangleold.Block, lastConfirmedEpoch epoch.Index) {
	s, ok := m.solidEntryPoints[b.EI()]
	if b.EI() < lastConfirmedEpoch || !ok {
		return
	}

	bytes, _ := b.Bytes()
	s.Delete(bytes)

	return
}

// DumpSolidEntryPoints dumps solid entry points within given epochs.
func (m *Manager) DumpSolidEntryPoints(lastConfirmedEpoch, latestCommittableEpoch epoch.Index) (seps map[epoch.Index][]tangleold.BlockID) {
	seps = make(map[epoch.Index][]tangleold.BlockID)

	for i := lastConfirmedEpoch; i < latestCommittableEpoch; i++ {
		sep, ok := m.solidEntryPoints[i]
		if ok {
			sep.ForEach(func(_ []byte, cachedBlock *objectstorage.CachedObject[*tangleold.Block]) bool {
				cachedBlock.Consume(func(b *tangleold.Block) {
					seps[i] = append(seps[i], b.ID())
				})
				return true
			})
		}
	}

	return
}

func (m *Manager) Shutdown() {
	m.shutdownOnce.Do(func() {
		for _, storage := range m.solidEntryPoints {
			storage.Shutdown()
		}
	})
}

// options is a container for all configurable parameters of the Indexer.
type options struct {
	// cacheTimeProvider contains the cacheTimeProvider that overrides the local cache times.
	cacheTimeProvider *database.CacheTimeProvider

	solidEntryPointCacheTime time.Duration
}

// newOptions returns a new options object that corresponds to the handed in options and which is derived from the
// default options.
func newOptions(option ...Option) (new *options) {
	return (&options{
		cacheTimeProvider:        database.NewCacheTimeProvider(0),
		solidEntryPointCacheTime: 10 * time.Second,
	}).apply(option...)
}

// apply modifies the options object by overriding the handed in options.
func (o *options) apply(options ...Option) (self *options) {
	for _, option := range options {
		option(o)
	}

	return o
}

// SolidEntryPointCacheTime is an Option for the Ledger that allows to configure which KVStore is supposed to be used to persist data
// (the default option is to use a MapDB).
func SolidEntryPointCacheTime(duration time.Duration) (option Option) {
	return func(options *options) {
		options.solidEntryPointCacheTime = duration
	}
}

// Option represents the return type of optional parameters that can be handed into the constructor of the EpochStateDiffStorage
// to configure its behavior.
type Option func(*options)
