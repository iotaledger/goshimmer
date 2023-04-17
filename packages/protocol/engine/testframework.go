package engine

import (
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/clock"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/filter"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/module"
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
	MemPool       *mempool.TestFramework
	VirtualVoting *booker.VirtualVotingTestFramework
	Acceptance    *blockgadget.TestFramework
}

func NewTestEngine(t *testing.T, workers *workerpool.Group, storage *storage.Storage,
	clock module.Provider[*Engine, clock.Clock],
	ledger module.Provider[*Engine, ledger.Ledger],
	filter module.Provider[*Engine, filter.Filter],
	sybilProtection module.Provider[*Engine, sybilprotection.SybilProtection],
	throughputQuota module.Provider[*Engine, throughputquota.ThroughputQuota],
	notarization module.Provider[*Engine, notarization.Notarization],
	tangle module.Provider[*Engine, tangle.Tangle],
	consensus module.Provider[*Engine, consensus.Consensus],
	opts ...options.Option[Engine],
) *Engine {
	e := New(workers.CreateGroup("Engine"),
		storage,
		clock,
		ledger,
		filter,
		sybilProtection,
		throughputQuota,
		notarization,
		tangle,
		consensus,
		opts...,
	)
	t.Cleanup(e.Shutdown)
	return e
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, engine *Engine) *TestFramework {
	t := &TestFramework{
		test:     test,
		Instance: engine,
		Booker:   booker.NewTestFramework(test, workers.CreateGroup("BookerTestFramework"), engine.Tangle.Booker(), engine.Tangle.BlockDAG(), engine.Ledger.MemPool(), engine.SybilProtection.Validators(), engine.SlotTimeProvider),
	}
	t.Tangle = tangle.NewTestFramework(test, engine.Tangle, t.Booker)
	t.Acceptance = blockgadget.NewTestFramework(test,
		engine.Consensus.BlockGadget(),
		t.Tangle,
	)
	t.MemPool = t.Tangle.MemPool
	t.BlockDAG = t.Tangle.BlockDAG
	t.VirtualVoting = t.Tangle.VirtualVoting
	return t
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group,
	clock module.Provider[*Engine, clock.Clock],
	ledger module.Provider[*Engine, ledger.Ledger],
	filter module.Provider[*Engine, filter.Filter],
	sybilProtection module.Provider[*Engine, sybilprotection.SybilProtection],
	throughputQuota module.Provider[*Engine, throughputquota.ThroughputQuota],
	notarization module.Provider[*Engine, notarization.Notarization],
	tangle module.Provider[*Engine, tangle.Tangle],
	consensus module.Provider[*Engine, consensus.Consensus],
	optsEngine ...options.Option[Engine],
) *TestFramework {
	engine := NewTestEngine(t, workers.CreateGroup("Engine"),
		blockdag.NewTestStorage(t, workers, database.WithDBProvider(database.NewDB)),
		clock,
		ledger,
		filter,
		sybilProtection,
		throughputQuota,
		notarization,
		tangle,
		consensus,
		optsEngine...,
	)
	t.Cleanup(engine.Shutdown)

	return NewTestFramework(t, workers, engine)
}

func (e *TestFramework) AssertSlotState(index slot.Index) {
	require.Equal(e.test, index, e.Instance.Storage.Settings.LatestCommitment().Index(), "last commitment index is not equal")
	require.Equal(e.test, index, e.Instance.Notarization.Attestations().LastCommittedSlot(), "notarization manager attestations last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.Ledger.UnspentOutputs().LastCommittedSlot(), "ledger unspent outputs last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.SybilProtection.LastCommittedSlot(), "sybil protection last committed slot is not equal")
	// TODO: throughput quota is not updated with each slot, but with acceptance
	// require.Equal(e.test, index, e.Engine.ThroughputQuota.(*mana1.ThroughputQuota).LastCommittedSlot(), "throughput quota last committed slot is not equal")
	require.Equal(e.test, index, e.Instance.EvictionState.LastEvictedSlot(), "last evicted slot is not equal")
}

func (e *TestFramework) AssertRootBlocks(rootBlocks []*models.Block) {
	for _, rootBlock := range rootBlocks {
		rootBlockID := rootBlock.ID()
		require.True(e.test, e.Instance.EvictionState.IsRootBlock(rootBlockID), "root block is not in eviction state %s", rootBlockID)
		require.True(e.test, lo.PanicOnErr(e.Instance.Storage.RootBlocks.Has(rootBlockID)), "root block is not in storage %s", rootBlockID)
	}
}

func (e *TestFramework) SlotTimeProvider() *slot.TimeProvider {
	return e.Instance.SlotTimeProvider()
}

func (e *TestFramework) ExportBytes(export func(io.WriteSeeker, slot.Index) error, targetIndex slot.Index) []byte {
	w := new(WriteSeekerBuffer)
	require.NoError(e.test, export(w, targetIndex))
	return w.Bytes()
}

type WriteSeekerBuffer struct {
	bytes.Buffer
	position int64
}

func (wsb *WriteSeekerBuffer) Write(p []byte) (n int, err error) {
	n, err = wsb.Buffer.Write(p)
	wsb.position += int64(n)
	return n, err
}

func (wsb *WriteSeekerBuffer) Seek(offset int64, whence int) (int64, error) {
	return 0, nil
}
