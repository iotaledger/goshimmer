package protocol

import (
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

		s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), DatabaseVersion)
		creator.CreateSnapshot(s, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
			identity.GenerateIdentity().ID(): 100,
		})

		t.Protocol = New(t.Network.Join(identity.GenerateIdentity().ID()), append(t.optsProtocolOptions, WithSnapshotPath(diskUtil.Path("snapshot.bin")), WithBaseDirectory(diskUtil.Path()))...)
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
