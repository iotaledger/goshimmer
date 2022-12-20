package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/dpos"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/throughputquota/mana1"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Network  *network.MockedNetwork
	Protocol *Protocol

	test *testing.T

	optsProtocolOptions []options.Option[Protocol]
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	_ = logger.InitGlobalLogger(configuration.New())

	return options.Apply(&TestFramework{
		Network: network.NewMockedNetwork(),

		test: test,
	}, opts, func(t *TestFramework) {
		diskUtil := diskutil.New(test.TempDir())

		test.Cleanup(func() {
			t.Protocol.Shutdown()
		})

		identitiesWeights := map[identity.ID]uint64{
			identity.New(ed25519.GenerateKeyPair().PublicKey).ID(): 100,
		}
		snapshotcreator.CreateSnapshot(DatabaseVersion, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32), identitiesWeights, lo.Keys(identitiesWeights))

		t.Protocol = New(t.Network.Join(identity.GenerateIdentity().ID()), append(t.optsProtocolOptions, WithSnapshotPath(diskUtil.Path("snapshot.bin")), WithBaseDirectory(diskUtil.Path()))...)
	})
}

// region EngineTestFramework //////////////////////////////////////////////////////////////////////////////////////////

type (
	TangleTestFramework     = tangle.TestFramework
	AcceptanceTestFramework = blockgadget.TestFramework
)

type EngineTestFramework struct {
	Engine *engine.Engine

	test *testing.T

	optsStorage       *storage.Storage
	optsTangleOptions []options.Option[tangle.Tangle]

	Tangle     *TangleTestFramework
	Acceptance *AcceptanceTestFramework
}

func NewEngineTestFramework(test *testing.T, opts ...options.Option[EngineTestFramework]) (testFramework *EngineTestFramework) {
	return options.Apply(&EngineTestFramework{
		test: test,
	}, opts, func(t *EngineTestFramework) {
		if t.Engine == nil {
			if t.optsStorage == nil {
				t.optsStorage = storage.New(t.test.TempDir(), 1)
				test.Cleanup(t.optsStorage.Shutdown)
			}

			t.Engine = engine.New(t.optsStorage, dpos.NewProvider(), mana1.NewProvider(), engine.WithTangleOptions(t.optsTangleOptions...))
			test.Cleanup(t.Engine.Shutdown)
		}

		t.Tangle = tangle.NewTestFramework(test, tangle.WithTangle(t.Engine.Tangle))
		t.Acceptance = blockgadget.NewTestFramework(test,
			blockgadget.WithGadget(t.Engine.Consensus.BlockGadget),
			blockgadget.WithTangle(t.Engine.Tangle),
			blockgadget.WithTangleTestFramework(t.Tangle),
			blockgadget.WithEvictionState(t.Engine.EvictionState),
		)
	})
}

func (e *EngineTestFramework) AssertEpochState(index epoch.Index) {
	require.Equal(e.test, index, e.Engine.Storage.Settings.LatestCommitment().Index())
	require.Equal(e.test, index, e.Engine.NotarizationManager.Attestations.LastCommittedEpoch())
	require.Equal(e.test, index, e.Engine.LedgerState.UnspentOutputs.LastCommittedEpoch())
	require.Equal(e.test, index, e.Engine.SybilProtection.(*dpos.SybilProtection).LastCommittedEpoch())
	require.Equal(e.test, index, e.Engine.ThroughputQuota.(*mana1.ThroughputQuota).LastCommittedEpoch())
	require.Equal(e.test, index, e.Engine.EvictionState.LastEvictedEpoch())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngine(engine *engine.Engine) options.Option[EngineTestFramework] {
	return func(t *EngineTestFramework) {
		t.Engine = engine
	}
}

func WithStorage(storageInstance *storage.Storage) options.Option[EngineTestFramework] {
	return func(t *EngineTestFramework) {
		t.optsStorage = storageInstance
	}
}

func WithTangleOptions(tangleOpts ...options.Option[tangle.Tangle]) options.Option[EngineTestFramework] {
	return func(t *EngineTestFramework) {
		t.optsTangleOptions = tangleOpts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
