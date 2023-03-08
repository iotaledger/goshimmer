package engine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/module"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type TestFramework struct {
	Instance *Engine

	test *testing.T

	optsStorage       *storage.Storage
	optsTangleOptions []options.Option[tangle.Tangle]

	Tangle        *tangle.TestFramework
	Booker        *booker.TestFramework
	BlockDAG      *blockdag.TestFramework
	Ledger        *ledger.TestFramework
	VirtualVoting *virtualvoting.TestFramework
	Acceptance    *blockgadget.TestFramework
}

func NewTestEngine(t *testing.T, workers *workerpool.Group, storage *storage.Storage,
	clock module.Provider[*Engine, clock.Clock],
	ledger module.Provider[*Engine, ledger.Ledger],
	ledgerState module.Provider[*Engine, ledgerstate.LedgerState],
	sybilProtection module.Provider[*Engine, sybilprotection.SybilProtection],
	throughputQuota module.Provider[*Engine, throughputquota.ThroughputQuota],
	opts ...options.Option[Engine]) *Engine {
	e := New(workers.CreateGroup("Engine"), storage, clock, ledger, ledgerState, sybilProtection, throughputQuota, opts...)
	t.Cleanup(e.Shutdown)
	return e
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, engine *Engine) *TestFramework {
	t := &TestFramework{
		test:     test,
		Instance: engine,
		Tangle:   tangle.NewTestFramework(test, engine.Tangle, booker.NewTestFramework(test, workers.CreateGroup("BookerTestFramework"), engine.Tangle.Booker)),
	}
	t.Acceptance = blockgadget.NewTestFramework(test,
		engine.Consensus.BlockGadget,
		t.Tangle,
	)
	t.Booker = t.Tangle.Booker
	t.Ledger = t.Tangle.Ledger
	t.BlockDAG = t.Tangle.BlockDAG
	t.VirtualVoting = t.Tangle.VirtualVoting
	return t
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group,
	clock module.Provider[*Engine, clock.Clock],
	ledger module.Provider[*Engine, ledger.Ledger],
	ledgerState module.Provider[*Engine, ledgerstate.LedgerState],
	sybilProtection module.Provider[*Engine, sybilprotection.SybilProtection],
	throughputQuota module.Provider[*Engine, throughputquota.ThroughputQuota],
	optsEngine ...options.Option[Engine]) *TestFramework {
	engine := NewTestEngine(t, workers.CreateGroup("Engine"), blockdag.NewTestStorage(t, workers, database.WithDBProvider(database.NewDB)), clock, ledger, ledgerState, sybilProtection, throughputQuota, optsEngine...)
	t.Cleanup(engine.Shutdown)

	return NewTestFramework(t, workers, engine)
}

func (e *TestFramework) AssertSlotState(index slot.Index) {
	require.Equal(e.test, index, e.Instance.Storage.Settings.LatestCommitment().Index(), "last commitment index is not equal")
	require.Equal(e.test, index, e.Instance.NotarizationManager.Attestations.LastCommittedSlot(), "notarization manager attestations last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.LedgerState.UnspentOutputs().LastCommittedSlot(), "ledger state unspent outputs last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.SybilProtection.LastCommittedSlot(), "sybil protection last committed slot is not equal")
	// TODO: throughput quota is not updated with each slot, but with acceptance
	// require.Equal(e.test, index, e.Engine.ThroughputQuota.(*mana1.ThroughputQuota).LastCommittedSlot(), "throughput quota last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.EvictionState.LastEvictedSlot(), "last evicted slot is not equal")
}

func (e *TestFramework) SlotTimeProvider() *slot.TimeProvider {
	return e.Instance.SlotTimeProvider()
}
