package mana1

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/module"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	PrefixLastCommittedSlot byte = iota
	PrefixTotalBalance
	PrefixQuotasByID
)

// ThroughputQuota is the manager that tracks the throughput quota of identities according to mana1 (delegated pledge).
type ThroughputQuota struct {
	engine              *engine.Engine
	workers             *workerpool.Group
	quotaByIDStorage    *kvstore.TypedStore[identity.ID, storable.SerializableInt64, *identity.ID, *storable.SerializableInt64]
	quotaByIDCache      *shrinkingmap.ShrinkingMap[identity.ID, int64]
	quotaByIDMutex      sync.RWMutex // TODO: replace this lock with DAG mutex so each entity is individually locked
	totalBalanceStorage kvstore.KVStore
	totalBalance        int64
	totalBalanceMutex   sync.RWMutex

	traits.BatchCommittable
	module.Module
}

// New creates a new ThroughputQuota manager.
func New(engineInstance *engine.Engine, opts ...options.Option[ThroughputQuota]) (manaTracker *ThroughputQuota) {
	return options.Apply(&ThroughputQuota{
		BatchCommittable:    traits.NewBatchCommittable(engineInstance.Storage.ThroughputQuota(), PrefixLastCommittedSlot),
		engine:              engineInstance,
		workers:             engineInstance.Workers.CreateGroup("ThroughputQuota"),
		totalBalanceStorage: engineInstance.Storage.ThroughputQuota(PrefixTotalBalance),
		quotaByIDStorage:    kvstore.NewTypedStore[identity.ID, storable.SerializableInt64](engineInstance.Storage.ThroughputQuota(PrefixQuotasByID)),
		quotaByIDCache:      shrinkingmap.New[identity.ID, int64](),
	}, opts, func(m *ThroughputQuota) {
		if iterationErr := m.quotaByIDStorage.Iterate([]byte{}, func(key identity.ID, value storable.SerializableInt64) (advance bool) {
			m.quotaByIDMutex.Lock()
			defer m.quotaByIDMutex.Unlock()

			m.updateMana(key, int64(value))
			return true
		}); iterationErr != nil {
			panic(iterationErr)
		}

		if totalBalanceBytes, err := m.totalBalanceStorage.Get([]byte{0}); err == nil {
			totalBalance := new(storable.SerializableInt64)
			if _, err := totalBalance.FromBytes(totalBalanceBytes); err != nil {
				panic(err)
			}
			m.totalBalance = int64(*totalBalance)
		}

		m.engine.HookConstructed(func() {
			m.engine.Storage.Settings.HookInitialized(func() {
				m.SetLastCommittedSlot(m.engine.Storage.Settings.LatestCommitment().Index())
			})

			m.engine.Ledger.UnspentOutputs().Subscribe(m)
			m.engine.Ledger.HookInitialized(m.init)
		})
	})
}

// NewProvider returns a new throughput quota provider that uses mana1.
func NewProvider(opts ...options.Option[ThroughputQuota]) module.Provider[*engine.Engine, throughputquota.ThroughputQuota] {
	return module.Provide(func(e *engine.Engine) throughputquota.ThroughputQuota {
		return New(e, opts...)
	})
}

// Balance returns the balance of the given identity.
func (m *ThroughputQuota) Balance(id identity.ID) (mana int64, exists bool) {
	m.quotaByIDMutex.RLock()
	defer m.quotaByIDMutex.RUnlock()

	return m.quotaByIDCache.Get(id)
}

// BalanceByIDs returns the balances of all known identities.
func (m *ThroughputQuota) BalanceByIDs() (manaByID map[identity.ID]int64) {
	m.quotaByIDMutex.RLock()
	defer m.quotaByIDMutex.RUnlock()

	return m.quotaByIDCache.AsMap()
}

// TotalBalance returns the total amount of throughput quota.
func (m *ThroughputQuota) TotalBalance() (totalMana int64) {
	m.totalBalanceMutex.RLock()
	defer m.totalBalanceMutex.RUnlock()

	return m.totalBalance
}

func (m *ThroughputQuota) ApplyCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	m.quotaByIDMutex.Lock()
	defer m.quotaByIDMutex.Unlock()

	return m.applyCreatedOutput(output)
}

func (m *ThroughputQuota) applyCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), int64(iotaBalance))

		if !m.engine.Ledger.UnspentOutputs().WasInitialized() {
			totalBalanceBytes, serializationErr := storable.SerializableInt64(m.updateTotalBalance(int64(iotaBalance))).Bytes()
			if serializationErr != nil {
				return errors.Wrapf(serializationErr, "failed to serialize total balance")
			}

			if err := m.totalBalanceStorage.Set([]byte{0}, totalBalanceBytes); err != nil {
				return errors.Wrap(err, "failed to persist total balance")
			}
		}
	}

	return
}

func (m *ThroughputQuota) ApplySpentOutput(output *mempool.OutputWithMetadata) (err error) {
	m.quotaByIDMutex.Lock()
	defer m.quotaByIDMutex.Unlock()

	return m.applySpentOutput(output)
}

func (m *ThroughputQuota) applySpentOutput(output *mempool.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), -int64(iotaBalance))
	}

	return
}

func (m *ThroughputQuota) RollbackCreatedOutput(output *mempool.OutputWithMetadata) (err error) {
	return m.ApplySpentOutput(output)
}

func (m *ThroughputQuota) RollbackSpentOutput(output *mempool.OutputWithMetadata) (err error) {
	return m.ApplyCreatedOutput(output)
}

func (m *ThroughputQuota) BeginBatchedStateTransition(targetSlot slot.Index) (currentSlot slot.Index, err error) {
	return m.BatchCommittable.BeginBatchedStateTransition(targetSlot)
}

func (m *ThroughputQuota) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())

	m.FinalizeBatchedStateTransition()

	done()

	return
}

func (m *ThroughputQuota) init() {
	m.engine.Ledger.UnspentOutputs().Unsubscribe(m)

	m.TriggerInitialized()

	wp := m.workers.CreatePool("ThroughputQuota", 2)
	m.engine.Ledger.MemPool().Events().TransactionAccepted.Hook(func(event *mempool.TransactionEvent) {
		m.quotaByIDMutex.Lock()
		defer m.quotaByIDMutex.Unlock()
		for _, createdOutput := range event.CreatedOutputs {
			if createdOutputErr := m.applyCreatedOutput(createdOutput); createdOutputErr != nil {
				panic(createdOutputErr)
			}
		}

		for _, spentOutput := range event.SpentOutputs {
			if spentOutputErr := m.applySpentOutput(spentOutput); spentOutputErr != nil {
				panic(spentOutputErr)
			}
		}
	}, event.WithWorkerPool(wp))
	m.engine.Ledger.MemPool().Events().TransactionOrphaned.Hook(func(event *mempool.TransactionEvent) {
		m.quotaByIDMutex.Lock()
		defer m.quotaByIDMutex.Unlock()

		for _, createdOutput := range event.CreatedOutputs {
			if spentOutputErr := m.applySpentOutput(createdOutput); spentOutputErr != nil {
				panic(spentOutputErr)
			}
		}

		for _, spentOutput := range event.SpentOutputs {
			if createdOutputErr := m.applyCreatedOutput(spentOutput); createdOutputErr != nil {
				panic(createdOutputErr)
			}
		}
	}, event.WithWorkerPool(wp))
}

func (m *ThroughputQuota) updateMana(id identity.ID, diff int64) {
	if newBalance := lo.Return1(m.quotaByIDCache.Get(id)) + diff; newBalance != 0 {
		m.quotaByIDCache.Set(id, newBalance)

		if err := m.quotaByIDStorage.Set(id, storable.SerializableInt64(newBalance)); err != nil {
			panic(err)
		}
	} else {
		m.quotaByIDCache.Delete(id)
		if err := m.quotaByIDStorage.Delete(id); err != nil {
			panic(err)
		}
	}
}

func (m *ThroughputQuota) updateTotalBalance(delta int64) (newTotalBalance int64) {
	m.totalBalanceMutex.Lock()
	defer m.totalBalanceMutex.Unlock()

	newTotalBalance = m.totalBalance + delta
	m.totalBalance = newTotalBalance

	return
}
