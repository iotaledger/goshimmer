package ratesetter

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/configuration"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"

	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/stretchr/testify/assert"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	RateSetter    RateSetter
	test          *testing.T
	localIdentity *identity.Identity

	optsRateSetter []options.Option[Options]
	optsScheduler  []options.Option[scheduler.Scheduler]

	*ProtocolTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	_ = logger.InitGlobalLogger(configuration.New())
	rateSetterOpts := []options.Option[Options]{WithSchedulerRate(5 * time.Millisecond), WithMode(DisabledMode)}

	return options.Apply(&TestFramework{
		test:           test,
		optsRateSetter: rateSetterOpts,
	}, opts, func(t *TestFramework) {
		p := protocol.NewTestFramework(t.test, protocol.WithProtocolOptions(protocol.WithCongestionControlOptions(congestioncontrol.WithSchedulerOptions(t.optsScheduler...))))
		t.ProtocolTestFramework = p
		t.localIdentity = p.Local

		p.Protocol.Run()

		t.RateSetter = New(t.localIdentity.ID(), p.Protocol, t.optsRateSetter...)

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

func (tf *TestFramework) IssueBlock(block *models.Block) error {
	for {
		if estimate := tf.RateSetter.Estimate(); estimate > 0 {
			time.Sleep(estimate)
		} else {
			tf.Protocol.ProcessBlock(block, tf.localIdentity.ID())
			break
		}
	}
	return nil
}

type ProtocolTestFramework = protocol.TestFramework

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

//// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
