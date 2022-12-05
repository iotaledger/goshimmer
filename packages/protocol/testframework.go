package protocol

import (
	"os"
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/crypto/ed25519"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
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

		storageInstance := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), DatabaseVersion)
		test.Cleanup(func() {
			t.Protocol.Shutdown()
		})

		snapshotcreator.CreateSnapshot(storageInstance, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32), map[identity.ID]uint64{
			identity.New(ed25519.GenerateKeyPair().PublicKey).ID(): 100,
		})

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

	optsEngineOptions []options.Option[engine.Engine]

	Tangle     *TangleTestFramework
	Acceptance *AcceptanceTestFramework
}

func NewEngineTestFramework(test *testing.T, opts ...options.Option[EngineTestFramework]) (testFramework *EngineTestFramework) {
	chainStorage := storage.New(test.TempDir(), 1)
	return options.Apply(&EngineTestFramework{
		test: test,
	}, opts, func(t *EngineTestFramework) {
		if t.Engine == nil {
			t.Engine = engine.New(chainStorage, dpos.NewProvider(), mana1.NewProvider(), t.optsEngineOptions...)
		}

		t.Tangle = tangle.NewTestFramework(test, tangle.WithTangle(t.Engine.Tangle))
		t.Acceptance = blockgadget.NewTestFramework(test, blockgadget.WithTangle(t.Engine.Tangle), blockgadget.WithTangleTestFramework(t.Tangle))
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithEngine(engine *engine.Engine) options.Option[EngineTestFramework] {
	return func(t *EngineTestFramework) {
		t.Engine = engine
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
