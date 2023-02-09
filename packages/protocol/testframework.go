package protocol

import (
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/core/snapshotcreator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm"
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

		snapshotcreator.CreateSnapshot(DatabaseVersion, tempDir.Path("snapshot.bin"), genesisTokenAmount, make([]byte, ed25519.SeedSize), identitiesWeights, lo.Keys(identitiesWeights), ledgerVM)

		t.Instance = New(workers.CreateGroup("Protocol"), t.Network.Join(identity.GenerateIdentity().ID()), append(t.optsProtocolOptions, WithEngineOptions(engine.WithLedgerOptions(ledger.WithVM(ledgerVM))), WithSnapshotPath(tempDir.Path("snapshot.bin")), WithBaseDirectory(tempDir.Path()))...)

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
