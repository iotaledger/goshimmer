package mana1

import (
	"context"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	typedkvstore "github.com/iotaledger/hive.go/core/generics/kvstore"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/kvstore"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/storable"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
)

const (
	PrefixLastCommittedEpoch byte = iota
	PrefixTotalBalance
	PrefixQuotasByID
)

// ThroughputQuota is the manager that tracks the throughput quota of identities according to mana1 (delegated pledge).
type ThroughputQuota struct {
	engine              *engine.Engine
	quotaByIDStorage    *typedkvstore.TypedStore[identity.ID, storable.SerializableInt64, *identity.ID, *storable.SerializableInt64]
	quotaByIDCache      *shrinkingmap.ShrinkingMap[identity.ID, int64]
	quotaByIDMutex      sync.RWMutex
	totalBalanceStorage kvstore.KVStore
	totalBalance        int64
	totalBalanceMutex   sync.RWMutex

	traits.Initializable
	traits.BatchCommittable
}

// New creates a new ThroughputQuota manager.
func New(engineInstance *engine.Engine, opts ...options.Option[ThroughputQuota]) (manaTracker *ThroughputQuota) {
	return options.Apply(&ThroughputQuota{
		BatchCommittable:    traits.NewBatchCommittable(engineInstance.Storage.ThroughputQuota(PrefixLastCommittedEpoch)),
		Initializable:       traits.NewInitializable(),
		engine:              engineInstance,
		totalBalanceStorage: engineInstance.Storage.ThroughputQuota(PrefixTotalBalance),
		quotaByIDStorage:    typedkvstore.NewTypedStore[identity.ID, storable.SerializableInt64](engineInstance.Storage.ThroughputQuota(PrefixQuotasByID)),
		quotaByIDCache:      shrinkingmap.New[identity.ID, int64](),
	}, opts, func(m *ThroughputQuota) {
		m.quotaByIDStorage.Iterate([]byte{}, func(key identity.ID, value storable.SerializableInt64) (advance bool) {
			m.updateMana(key, int64(value))
			return true
		})

		if totalBalanceBytes, err := m.totalBalanceStorage.Get([]byte{0}); err == nil {
			totalBalance := new(storable.SerializableInt64)
			if _, err := totalBalance.FromBytes(totalBalanceBytes); err != nil {
				panic(err)
			}
			m.totalBalance = int64(*totalBalance)
		}

		m.engine.SubscribeConstructed(func() {
			m.engine.Storage.Settings.SubscribeInitialized(func() {
				m.SetLastCommittedEpoch(m.engine.Storage.Settings.LatestCommitment().Index())
			})

			m.engine.LedgerState.UnspentOutputs.Subscribe(m)
			m.engine.LedgerState.SubscribeInitialized(func() {
				m.engine.LedgerState.UnspentOutputs.Unsubscribe(m)

				m.TriggerInitialized()

				m.engine.Ledger.Events.TransactionAccepted.Attach(event.NewClosure(func(event *ledger.TransactionEvent) {
					for _, createdOutput := range event.CreatedOutputs {
						m.ApplyCreatedOutput(createdOutput)
					}

					for _, spentOutput := range event.SpentOutputs {
						m.ApplySpentOutput(spentOutput)
					}
				}))

				m.engine.Ledger.Events.TransactionOrphaned.Attach(event.NewClosure(func(event *ledger.TransactionEvent) {
					for _, createdOutput := range event.CreatedOutputs {
						m.ApplySpentOutput(createdOutput)
					}

					for _, spentOutput := range event.SpentOutputs {
						m.ApplyCreatedOutput(spentOutput)
					}
				}))
			})
		})
	})
}

// NewProvider returns a new throughput quota provider that uses mana1.
func NewProvider(opts ...options.Option[ThroughputQuota]) engine.ModuleProvider[throughputquota.ThroughputQuota] {
	return engine.ProvideModule(func(e *engine.Engine) throughputquota.ThroughputQuota {
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

func (m *ThroughputQuota) ApplyCreatedOutput(output *ledger.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), int64(iotaBalance))

		if !m.engine.LedgerState.UnspentOutputs.WasInitialized() {
			m.totalBalance += int64(iotaBalance)
			serializableTotalBalance := storable.SerializableInt64(m.totalBalance)
			totalBalanceBytes, serializationErr := serializableTotalBalance.Bytes()
			if serializationErr != nil {
				return errors.Errorf("failed to serialize total balance: %w", serializationErr)
			}

			if err := m.totalBalanceStorage.Set([]byte{0}, totalBalanceBytes); err != nil {
				return errors.Errorf("failed to persist total balance: %w", err)
			}
		}
	}

	return
}

func (m *ThroughputQuota) ApplySpentOutput(output *ledger.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), -int64(iotaBalance))
	}

	return
}

func (m *ThroughputQuota) RollbackCreatedOutput(output *ledger.OutputWithMetadata) (err error) {
	return m.ApplySpentOutput(output)
}

func (m *ThroughputQuota) RollbackSpentOutput(output *ledger.OutputWithMetadata) (err error) {
	return m.ApplyCreatedOutput(output)
}

func (m *ThroughputQuota) BeginBatchedStateTransition(targetEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	return m.BatchCommittable.BeginBatchedStateTransition(targetEpoch)
}

func (m *ThroughputQuota) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())

	m.FinalizeBatchedStateTransition()

	done()

	return
}

func (m *ThroughputQuota) updateMana(id identity.ID, diff int64) {
	m.quotaByIDMutex.Lock()
	defer m.quotaByIDMutex.Unlock()

	if newBalance := lo.Return1(m.quotaByIDCache.Get(id)) + diff; newBalance > 0 {
		m.quotaByIDCache.Set(id, newBalance)
		m.quotaByIDStorage.Set(id, storable.SerializableInt64(newBalance))
	} else {
		m.quotaByIDCache.Delete(id)
		m.quotaByIDStorage.Delete(id)
	}
}
