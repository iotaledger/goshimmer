package mana2

import (
	"context"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
)

// ManaTracker is the manager that tracks the mana balances of identities.
type ManaTracker struct {
	engine            *engine.Engine
	commitmentState   *ledgerstate.CommitmentState
	manaByID          *shrinkingmap.ShrinkingMap[identity.ID, int64]
	manaByIDMutex     sync.RWMutex
	totalBalance      int64
	totalBalanceMutex sync.RWMutex
}

// New creates a new ManaTracker.
func New(engineInstance *engine.Engine, opts ...options.Option[ManaTracker]) (manaTracker *ManaTracker) {
	return options.Apply(&ManaTracker{
		engine:          engineInstance,
		commitmentState: ledgerstate.NewCommitmentState(),
		manaByID:        shrinkingmap.New[identity.ID, int64](),
	}, opts)
}

// NewThroughputQuotaProvider returns a new throughput quota provider that uses mana1.
func NewThroughputQuotaProvider(opts ...options.Option[ManaTracker]) engine.ModuleProvider[throughputquota.ThroughputQuota] {
	return engine.ProvideModule(func(e *engine.Engine) throughputquota.ThroughputQuota {
		return New(e, opts...)
	})
}

func (m *ManaTracker) Init() {
	m.engine.Storage.Settings.Initialized.Attach(event.NewClosure(func(bool) {
		m.commitmentState.SetLastCommittedEpoch(m.engine.Storage.Settings.LatestCommitment().Index())
	}))

	m.engine.LedgerState.UnspentOutputs.Subscribe(m)
	m.engine.LedgerState.Initialized.Attach(event.NewClosure(func(bool) {
		m.engine.LedgerState.UnspentOutputs.Unsubscribe(m)

		// TODO: ATTACH TO EVENTS FROM MEMPOOL
	}))
}

// Balance returns the balance of the given identity.
func (m *ManaTracker) Balance(id identity.ID) (mana int64, exists bool) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.Get(id)
}

// BalanceByIDs returns the balances of all known identities.
func (m *ManaTracker) BalanceByIDs() (manaByID map[identity.ID]int64) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.AsMap()
}

// TotalBalance returns the total amount of throughput quota.
func (m *ManaTracker) TotalBalance() (totalMana int64) {
	m.totalBalanceMutex.RLock()
	defer m.totalBalanceMutex.RUnlock()

	return m.totalBalance
}

func (m *ManaTracker) ApplyCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), int64(iotaBalance))

		if !m.engine.LedgerState.UnspentOutputs.Initialized.WasTriggered() {
			m.totalBalance += int64(iotaBalance)
		}
	}

	return
}

func (m *ManaTracker) ApplySpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), -int64(iotaBalance))
	}

	return
}

func (m *ManaTracker) RollbackCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return m.ApplySpentOutput(output)
}

func (m *ManaTracker) RollbackSpentOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	return m.ApplyCreatedOutput(output)
}

func (m *ManaTracker) BeginBatchedStateTransition(targetEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	return m.commitmentState.BeginBatchedStateTransition(targetEpoch)
}

func (m *ManaTracker) CommitBatchedStateTransition() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())

	m.commitmentState.FinalizeBatchedStateTransition()

	done()

	return
}

func (m *ManaTracker) updateMana(id identity.ID, diff int64) {
	m.manaByIDMutex.Lock()
	defer m.manaByIDMutex.Unlock()

	if newBalance := lo.Return1(m.manaByID.Get(id)) + diff; newBalance != 0 {
		m.manaByID.Set(id, newBalance)
	} else {
		m.manaByID.Delete(id)
	}
}
