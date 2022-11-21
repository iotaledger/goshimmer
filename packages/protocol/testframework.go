package protocol

import (
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"os"
	"testing"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Network  *network.MockedNetwork
	Protocol *Protocol
	Local    *identity.Identity

	test *testing.T

	optsProtocolOptions          []options.Option[Protocol]
	optsCongestionControlOptions []options.Option[congestioncontrol.CongestionControl]
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
			if err := storageInstance.Shutdown(); err != nil {
				test.Fatal(err)
			}
		})

		creator.CreateSnapshot(storageInstance, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
			identity.GenerateIdentity().ID(): 100,
		})
		t.Local = identity.GenerateIdentity()
		t.Protocol = New(
			t.Network.Join(t.Local.ID()),
			append(
				t.optsProtocolOptions,
				WithSnapshotPath(diskUtil.Path("snapshot.bin")),
				WithBaseDirectory(diskUtil.Path()),
				WithCongestionControlOptions(t.optsCongestionControlOptions...),
			)...,
		)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithProtocolOptions(opts ...options.Option[Protocol]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsProtocolOptions = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
