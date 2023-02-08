package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

const genesisTokenAmount = 100

type TestFramework struct {
	Network  *network.MockedNetwork
	Instance *Protocol
	Local    *identity.Identity

	test    *testing.T
	workers *workerpool.Group

	Engine *EngineTestFramework

	optsProtocolOptions          []options.Option[Protocol]
	optsCongestionControlOptions []options.Option[congestioncontrol.CongestionControl]
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, ledgerVM vm.VM, opts ...options.Option[TestFramework]) *TestFramework {
	_ = logger.InitGlobalLogger(configuration.New())

	return options.Apply(&TestFramework{
		Network: network.NewMockedNetwork(),

		test:    test,
		workers: workers,
	}, opts, func(t *TestFramework) {
		tempDir := utils.NewDirectory(test.TempDir())

		test.Cleanup(func() {
			t.Instance.Shutdown()
		})

		identitiesWeights := map[ed25519.PublicKey]uint64{
			ed25519.GenerateKeyPair().PublicKey: 100,
		}

		snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("snapshot.bin"), genesisTokenAmount, make([]byte, ed25519.SeedSize), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

		t.Instance = New(workers.CreateGroup("Protocol"), t.Network.Join(identity.GenerateIdentity().ID()), append(t.optsProtocolOptions, WithEngineOptions(engine.WithLedgerOptions(ledger.WithVM(ledgerVM))), WithSnapshotPath(tempDir.Path("snapshot.bin")), WithBaseDirectory(tempDir.Path()))...)

		t.Engine = NewEngineTestFramework(t.test, t.workers.CreateGroup("EngineTestFramework"), t.Instance.Engine())
	})
}

// region EngineTestFramework //////////////////////////////////////////////////////////////////////////////////////////

type EngineTestFramework struct {
	Engine *engine.Engine

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

func NewTestEngine(t *testing.T, workers *workerpool.Group, storage *storage.Storage, opts ...options.Option[engine.Engine]) *engine.Engine {
	e := engine.New(workers.CreateGroup("Engine"), storage, dpos.NewProvider(), mana1.NewProvider(), opts...)
	t.Cleanup(e.Shutdown)
	return e
}

func NewEngineTestFramework(test *testing.T, workers *workerpool.Group, engine *engine.Engine) *EngineTestFramework {
	t := &EngineTestFramework{
		test:   test,
		Engine: engine,
		Tangle: tangle.NewTestFramework(test, engine.Tangle, virtualvoting.NewTestFramework(test, workers.CreateGroup("VirtualVotingTestFramework"), engine.Tangle.VirtualVoting)),
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

func NewDefaultEngineTestFramework(t *testing.T, workers *workerpool.Group, optsEngine ...options.Option[engine.Engine]) *EngineTestFramework {
	engine := NewTestEngine(t, workers.CreateGroup("Engine"), blockdag.NewTestStorage(t, workers, database.WithDBProvider(database.NewDB)), optsEngine...)
	t.Cleanup(engine.Shutdown)

	return NewEngineTestFramework(t, workers, engine)
}

func (e *EngineTestFramework) AssertEpochState(index epoch.Index) {
	require.Equal(e.test, index, e.Engine.Storage.Settings.LatestCommitment().Index(), "last commitment index is not equal")
	require.Equal(e.test, index, e.Engine.NotarizationManager.Attestations.LastCommittedEpoch(), "notarization manager attestations last committed epoch is not equal")
	require.Equal(e.test, index, e.Engine.LedgerState.UnspentOutputs.LastCommittedEpoch(), "ledger state unspent outputs last committed epoch is not equal")
	require.Equal(e.test, index, e.Engine.SybilProtection.(*dpos.SybilProtection).LastCommittedEpoch(), "sybil protection last committed epoch is not equal")
	//TODO: throughput quota is not updated with each epoch, but with acceptance
	//require.Equal(e.test, index, e.Engine.ThroughputQuota.(*mana1.ThroughputQuota).LastCommittedEpoch(), "throughput quota last committed epoch is not equal")
	require.Equal(e.test, index, e.Engine.EvictionState.LastEvictedEpoch(), "last evicted epoch is not equal")
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithProtocolOptions(opts ...options.Option[Protocol]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsProtocolOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
