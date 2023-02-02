package ratesetter

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/workerpool"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test          *testing.T
	localIdentity []*identity.Identity
	RateSetter    []RateSetter

	optsRateSetter []options.Option[Options]
	optsScheduler  []options.Option[scheduler.Scheduler]
	optsNumIssuers int

	Protocol *protocol.TestFramework
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, opts ...options.Option[TestFramework]) *TestFramework {
	_ = logger.InitGlobalLogger(configuration.New())
	rateSetterOpts := []options.Option[Options]{WithSchedulerRate(5 * time.Millisecond), WithMode(DisabledMode)}

	return options.Apply(&TestFramework{
		test:           test,
		optsRateSetter: rateSetterOpts,
		optsNumIssuers: 1,
	}, opts, func(t *TestFramework) {
		p := protocol.NewTestFramework(t.test, workers.CreateGroup("Protocol"), protocol.WithProtocolOptions(protocol.WithCongestionControlOptions(congestioncontrol.WithSchedulerOptions(t.optsScheduler...))))
		t.Protocol = p
		p.Instance.Run()
		for i := 0; i < t.optsNumIssuers; i++ {
			localID := identity.GenerateIdentity()
			t.localIdentity = append(t.localIdentity, localID)
			t.RateSetter = append(t.RateSetter, New(localID.ID(), p.Instance, t.optsRateSetter...))
		}
	},
	)
}

func (tf *TestFramework) CreateBlock(issuer int) *models.Block {
	parents := models.NewParentBlockIDs()
	parents.AddStrong(models.EmptyBlockID)
	blk := models.NewBlock(models.WithIssuer(tf.localIdentity[issuer].PublicKey()), models.WithParents(parents))
	assert.NoError(tf.test, blk.DetermineID())
	return blk
}

func (tf *TestFramework) IssueBlock(block *models.Block, issuer int) error {
	for estimate := tf.RateSetter[issuer].Estimate(); estimate > 0; estimate = tf.RateSetter[issuer].Estimate() {
		time.Sleep(estimate)
	}

	return tf.Protocol.Instance.ProcessBlock(block, tf.localIdentity[issuer].ID())
}

func (tf *TestFramework) IssueBlocks(numBlocks int, issuer int) map[models.BlockID]*models.Block {
	blocks := make(map[models.BlockID]*models.Block, numBlocks)
	for i := 0; i < numBlocks; i++ {
		blk := tf.CreateBlock(issuer)
		blocks[blk.ID()] = blk
		assert.NoError(tf.test, tf.IssueBlock(blocks[blk.ID()], issuer))
	}
	return blocks
}

func (tf *TestFramework) Shutdown() {
	for _, rateSetter := range tf.RateSetter {
		rateSetter.Shutdown()
	}
	tf.Protocol.Instance.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithRateSetterOptions(opts ...options.Option[Options]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsRateSetter = append(tf.optsRateSetter, opts...)
	}
}

func WithSchedulerOptions(opts ...options.Option[scheduler.Scheduler]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsScheduler = opts
	}
}

func WithNumIssuers(numIssuers int) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsNumIssuers = numIssuers
	}
}

//// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
