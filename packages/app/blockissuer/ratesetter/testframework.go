package ratesetter

import (
	"os"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/core/diskutil"
	"github.com/iotaledger/goshimmer/packages/core/snapshot/creator"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/goshimmer/packages/storage"
	"github.com/stretchr/testify/assert"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	RateSetter    RateSetter
	Scheduler     *scheduler.Scheduler
	test          *testing.T
	localIdentity *identity.Identity

	optsRateSetterMode RateSetterModeType
	optsSchedulerRate  time.Duration
	optsMaxDeficit     int
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	_ = logger.InitGlobalLogger(configuration.New())

	return options.Apply(&TestFramework{
		test:               test,
		localIdentity:      identity.GenerateIdentity(),
		optsSchedulerRate:  5 * time.Millisecond,
		optsRateSetterMode: DisabledMode,
	}, opts, func(t *TestFramework) {
		diskUtil := diskutil.New(test.TempDir())

		s := storage.New(lo.PanicOnErr(os.MkdirTemp(os.TempDir(), "*")), protocol.DatabaseVersion)
		creator.CreateSnapshot(s, diskUtil.Path("snapshot.bin"), 100, make([]byte, 32, 32), map[identity.ID]uint64{
			identity.GenerateIdentity().ID(): 100,
		})

		localID := t.localIdentity.ID()
		protocol := protocol.New(network.NewMockedNetwork().Join(localID),
			protocol.WithSnapshotPath(diskUtil.Path("snapshot.bin")),
			protocol.WithBaseDirectory(diskUtil.Path()),
			protocol.WithCongestionControlOptions(congestioncontrol.WithSchedulerOptions(scheduler.WithRate(t.optsSchedulerRate), scheduler.WithMaxDeficit(t.optsMaxDeficit))),
		)
		protocol.Run()
		t.Scheduler = protocol.CongestionControl.Scheduler()
		switch t.optsRateSetterMode {
		case AIMDMode:
			t.RateSetter = NewAIMD(protocol, localID,
				WithSchedulerRateAIMD(t.optsSchedulerRate),
			)
		case DisabledMode:
			t.RateSetter = NewDisabled()
		default:
			t.RateSetter = NewDeficit(protocol, localID,
				WithSchedulerRateDeficit(t.optsSchedulerRate),
			)
		}
		t.RateSetter.Events().BlockIssued.Attach(event.NewClosure(func(block *models.Block) {
			protocol.ProcessBlock(block, t.localIdentity.ID())
		}))
	},
	)
}

func (tf *TestFramework) CreateBlock() *models.Block {
	parents := models.NewParentBlockIDs()
	parents.AddStrong(models.EmptyBlockID)
	blk := models.NewBlock(models.WithIssuer(tf.localIdentity.PublicKey()), models.WithParents(parents))
	assert.NoError(tf.test, blk.DetermineID())
	return blk
}

func (tf *TestFramework) SubmitBlocks(count int) {
	for i := 1; i <= count; i++ {
		blk := tf.CreateBlock()
		assert.NoError(tf.test, tf.RateSetter.SubmitBlock(blk))
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithSchedulerRate(rate time.Duration) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsSchedulerRate = rate
	}
}

func WithRateSetterMode(mode RateSetterModeType) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsRateSetterMode = mode
	}
}

func WithMaxDeficit(maxDef int) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsMaxDeficit = maxDef
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
