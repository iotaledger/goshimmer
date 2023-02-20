package protocol

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
	"github.com/iotaledger/goshimmer/packages/storage/utils"
	"github.com/iotaledger/hive.go/app/configuration"
	"github.com/iotaledger/hive.go/app/logger"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/stretchr/testify/require"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

const genesisTokenAmount = 100

type TestFramework struct {
	Network  *network.MockedNetwork
	Instance *Protocol
	Local    *identity.Identity

	test    *testing.T
	workers *workerpool.Group

	Engine *engine.TestFramework

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

		err := snapshotcreator.CreateSnapshot(
			snapshotcreator.WithDatabaseVersion(DatabaseVersion),
			snapshotcreator.WithVM(ledgerVM),
			snapshotcreator.WithFilePath(tempDir.Path("snapshot.bin")),
			snapshotcreator.WithGenesisTokenAmount(genesisTokenAmount),
			snapshotcreator.WithGenesisSeed(make([]byte, ed25519.SeedSize)),
			snapshotcreator.WithPledgeIDs(identitiesWeights),
			snapshotcreator.WithAttestAll(true),
		)
		require.NoError(test, err)

		t.Instance = New(workers.CreateGroup("Protocol"), t.Network.Join(identity.GenerateIdentity().ID()), append(t.optsProtocolOptions,
			WithSnapshotPath(tempDir.Path("snapshot.bin")),
			WithBaseDirectory(tempDir.Path()),
			WithEngineOptions(engine.WithLedgerOptions(ledger.WithVM(ledgerVM))),
		)...)

		t.Engine = engine.NewTestFramework(t.test, t.workers.CreateGroup("EngineTestFramework"), t.Instance.Engine())
	})
}

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithProtocolOptions(opts ...options.Option[Protocol]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsProtocolOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
