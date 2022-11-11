package ratesetter

import (
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/storage"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	RateSetter RateSetter
	test       *testing.T
	localID    identity.ID
}

var schedulerRate time.Duration = 10 * time.Millisecond

func NewTestFramework(test *testing.T, localID identity.ID, mode RateSetterModeType, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	_ = logger.InitGlobalLogger(configuration.New())
	var rateSetter RateSetter

	diskUtil := diskutil.New(test.TempDir())

	s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), protocol.DatabaseVersion)
	creator.CreateSnapshot(s, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
		identity.GenerateIdentity().ID(): 100,
	})

	protocol := protocol.New(network.NewMockedNetwork().Join(localID), protocol.WithSnapshotPath(diskUtil.Path("snapshot.bin")), protocol.WithBaseDirectory(diskUtil.Path()))

	switch mode {
	case AIMDMode:
		rateSetter = NewAIMD(protocol, localID,
			WithSchedulerRateAIMD(schedulerRate),
		)
	case DisabledMode:
		rateSetter = NewDisabled()
	default:
		rateSetter = NewDeficit(protocol, localID,
			WithSchedulerRateDeficit(schedulerRate),
		)
	}
	return &TestFramework{
		RateSetter: rateSetter,
		test:       test,
		localID:    localID,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
