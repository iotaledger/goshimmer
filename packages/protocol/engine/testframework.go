package engine

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
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

func NewTestEngine(t *testing.T, workers *workerpool.Group, storage *storage.Storage, sybilProtection ModuleProvider[sybilprotection.SybilProtection], throughputQuota ModuleProvider[throughputquota.ThroughputQuota], slotTimeProvider *slot.TimeProvider, opts ...options.Option[Engine]) *Engine {
	e := New(workers.CreateGroup("Engine"), storage, sybilProtection, throughputQuota, slotTimeProvider, opts...)
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

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, sybilProtection ModuleProvider[sybilprotection.SybilProtection], throughputQuota ModuleProvider[throughputquota.ThroughputQuota], slotTimeProvider *slot.TimeProvider, optsEngine ...options.Option[Engine]) *TestFramework {
	engine := NewTestEngine(t, workers.CreateGroup("Engine"), blockdag.NewTestStorage(t, workers, database.WithDBProvider(database.NewDB)), sybilProtection, throughputQuota, slotTimeProvider, optsEngine...)
	t.Cleanup(engine.Shutdown)

	return NewTestFramework(t, workers, engine)
}

func (e *TestFramework) AssertSlotState(index slot.Index) {
	require.Equal(e.test, index, e.Instance.Storage.Settings.LatestCommitment().Index(), "last commitment index is not equal")
	require.Equal(e.test, index, e.Instance.NotarizationManager.Attestations.LastCommittedSlot(), "notarization manager attestations last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.LedgerState.UnspentOutputs.LastCommittedSlot(), "ledger state unspent outputs last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.SybilProtection.LastCommittedSlot(), "sybil protection last committed slot is not equal")
	// TODO: throughput quota is not updated with each slot, but with acceptance
	// require.Equal(e.test, index, e.Engine.ThroughputQuota.(*mana1.ThroughputQuota).LastCommittedSlot(), "throughput quota last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.EvictionState.LastEvictedSlot(), "last evicted slot is not equal")
}
