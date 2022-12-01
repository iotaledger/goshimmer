package mana1

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/traits"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
)

// ThroughputQuota is the manager that tracks the throughput quota of identities according to mana1 (delegated pledge).
type ThroughputQuota struct {
	engine            *engine.Engine
	commitmentState   *ledgerstate.CommitmentState
	manaByID          *shrinkingmap.ShrinkingMap[identity.ID, int64]
	manaByIDMutex     sync.RWMutex
	totalBalance      int64
	totalBalanceMutex sync.RWMutex

	traits.Initializable
}

// New creates a new ThroughputQuota manager.
func New(engineInstance *engine.Engine, opts ...options.Option[ThroughputQuota]) (manaTracker *ThroughputQuota) {
	return options.Apply(&ThroughputQuota{
		Initializable:   traits.NewInitializable(),
		engine:          engineInstance,
		commitmentState: ledgerstate.NewCommitmentState(),
		manaByID:        shrinkingmap.New[identity.ID, int64](),
	}, opts, func(m *ThroughputQuota) {
		m.engine.SubscribeStartup(func() {
			m.engine.Storage.Settings.SubscribeInitialized(func() {
				m.commitmentState.SetLastCommittedEpoch(m.engine.Storage.Settings.LatestCommitment().Index())
			})

			m.engine.LedgerState.UnspentOutputs.Subscribe(m)
			m.engine.LedgerState.SubscribeInitialized(func() {
				m.engine.LedgerState.UnspentOutputs.Unsubscribe(m)

				m.TriggerInitialized()

				// TODO: ATTACH TO EVENTS FROM MEMPOOL
			})
		})
	})
}

// NewThroughputQuotaProvider returns a new throughput quota provider that uses mana1.
func NewThroughputQuotaProvider(opts ...options.Option[ThroughputQuota]) engine.ModuleProvider[throughputquota.ThroughputQuota] {
	return engine.ProvideModule(func(e *engine.Engine) throughputquota.ThroughputQuota {
		return New(e, opts...)
	})
}

// Balance returns the balance of the given identity.
func (m *ThroughputQuota) Balance(id identity.ID) (mana int64, exists bool) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.Get(id)
}

// BalanceByIDs returns the balances of all known identities.
func (m *ThroughputQuota) BalanceByIDs() (manaByID map[identity.ID]int64) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.AsMap()
}

// TotalBalance returns the total amount of throughput quota.
func (m *ThroughputQuota) TotalBalance() (totalMana int64) {
	m.totalBalanceMutex.RLock()
	defer m.totalBalanceMutex.RUnlock()

	return m.totalBalance
}

func (m *ThroughputQuota) ApplyCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), int64(iotaBalance))

		if !m.engine.LedgerState.UnspentOutputs.WasInitialized() {
			m.totalBalance += int64(iotaBalance)
		}
	}

	return
}

func (m *ThroughputQuota) ApplySpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), -int64(iotaBalance))
	}

	return
}

func (m *ThroughputQuota) RollbackCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return m.ApplySpentOutput(output)
}

func (m *ThroughputQuota) RollbackSpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return m.ApplyCreatedOutput(output)
}

func (m *ThroughputQuota) BeginBatchedStateTransition(targetEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	return m.commitmentState.BeginBatchedStateTransition(targetEpoch)
}

func (m *ThroughputQuota) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())

	m.commitmentState.FinalizeBatchedStateTransition()

	done()

	return
}

func (m *ThroughputQuota) updateMana(id identity.ID, diff int64) {
	m.manaByIDMutex.Lock()
	defer m.manaByIDMutex.Unlock()

	if newBalance := lo.Return1(m.manaByID.Get(id)) + diff; newBalance != 0 {
		m.manaByID.Set(id, newBalance)
	} else {
		m.manaByID.Delete(id)
	}
}
