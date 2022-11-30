package manatracker

import (
	"context"
	"errors"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
)

// ManaTracker is the manager that tracks the mana balances of identities.
type ManaTracker struct {
	manaByID                     *shrinkingmap.ShrinkingMap[identity.ID, int64]
	manaByIDMutex                sync.RWMutex
	totalMana                    int64
	ledgerState                  *ledgerstate.LedgerState
	initialized                  bool
	initializedMutex             sync.RWMutex
	lastCommittedEpochIndex      epoch.Index
	lastCommittedEpochIndexMutex sync.RWMutex
	batchEpochIndex              epoch.Index
	batchEpochIndexMutex         sync.RWMutex
}

// New creates a new ManaTracker.
func New(ledgerState *ledgerstate.LedgerState, opts ...options.Option[ManaTracker]) (manaTracker *ManaTracker) {
	return options.Apply(&ManaTracker{
		manaByID:    shrinkingmap.New[identity.ID, int64](),
		ledgerState: ledgerState,
	}, opts, func(m *ManaTracker) {
		ledgerState.UnspentOutputs.Subscribe(m)
	})
}

func (m *ManaTracker) Begin(committedEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	return m.setBatchEpoch(committedEpoch)
}

func (m *ManaTracker) Unsubscribe() {
	m.initializedMutex.Lock()
	defer m.initializedMutex.Unlock()

	m.ledgerState.UnspentOutputs.Unsubscribe(m)
	m.initialized = true
}

func (m *ManaTracker) Commit() (ctx context.Context) {
	ctx, done := context.WithCancel(context.Background())

	m.setLastCommittedEpoch(m.batchEpoch())
	m.setBatchEpoch(0)

	done()

	return
}

func (m *ManaTracker) ApplyCreatedOutput(output *ledgerstate.OutputWithMetadata) (err error) {
	m.initializedMutex.RLock()
	defer m.initializedMutex.RUnlock()

	if iotaBalance, exists := output.IOTABalance(); exists {
		m.updateMana(output.AccessManaPledgeID(), int64(iotaBalance))
		if !m.initialized {
			m.totalMana += int64(iotaBalance)
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

// Mana returns the mana balance of an identity.
func (m *ManaTracker) Mana(id identity.ID) (mana int64, exists bool) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.Get(id)
}

// ManaByIDs returns the mana balances of all identities.
func (m *ManaTracker) ManaByIDs() (manaByID map[identity.ID]int64) {
	m.manaByIDMutex.RLock()
	defer m.manaByIDMutex.RUnlock()

	return m.manaByID.AsMap()
}

// TotalMana returns the total amount of mana.
func (m *ManaTracker) TotalMana() (totalMana int64) {
	return m.totalMana
}

// updateMana adds the diff to the current mana balance of an identity .
func (m *ManaTracker) updateMana(id identity.ID, diff int64) {
	m.manaByIDMutex.Lock()
	defer m.manaByIDMutex.Unlock()

	if newBalance := lo.Return1(m.manaByID.Get(id)) + diff; newBalance != 0 {
		m.manaByID.Set(id, newBalance)
	} else {
		m.manaByID.Delete(id)
	}
}

func (m *ManaTracker) batchEpoch() (currentEpoch epoch.Index) {
	m.batchEpochIndexMutex.RLock()
	defer m.batchEpochIndexMutex.RUnlock()

	return m.batchEpochIndex
}

func (m *ManaTracker) setBatchEpoch(newEpoch epoch.Index) (currentEpoch epoch.Index, err error) {
	m.batchEpochIndexMutex.Lock()
	defer m.batchEpochIndexMutex.Unlock()

	if newEpoch != 0 && m.batchEpochIndex != 0 {
		return 0, errors.New("batch is already in progress")
	} else if (newEpoch - currentEpoch).Abs() > 1 {
		return 0, errors.New("batches can only be applied in order")
	} else if currentEpoch = m.lastCommittedEpoch(); currentEpoch != newEpoch {
		m.batchEpochIndex = newEpoch
	}

	return
}

func (m *ManaTracker) lastCommittedEpoch() (lastCommittedEpoch epoch.Index) {
	m.lastCommittedEpochIndexMutex.RLock()
	defer m.lastCommittedEpochIndexMutex.RUnlock()

	return m.lastCommittedEpochIndex
}

func (m *ManaTracker) setLastCommittedEpoch(index epoch.Index) {
	m.lastCommittedEpochIndexMutex.Lock()
	defer m.lastCommittedEpochIndexMutex.Unlock()

	m.lastCommittedEpochIndex = index
}
