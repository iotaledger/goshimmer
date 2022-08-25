package acceptancegadget

import (
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangle/otv"
	"github.com/iotaledger/goshimmer/packages/core/votes"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	evictionManager *eviction.Manager[models.BlockID]
	gadget          *AcceptanceGadget
	acceptedBlocks  uint32

	optsGadget         []options.Option[AcceptanceGadget]
	optsOnTangleVoting []options.Option[otv.OnTangleVoting]

	t *testing.T

	*otv.TestFramework
}

func NewTestFramework(t *testing.T, opts ...options.Option[TestFramework]) (tf *TestFramework) {
	tf = options.Apply(&TestFramework{
		t: t,
	}, opts)

	tf.TestFramework = otv.NewTestFramework(t, otv.WithOnTangleVotingOptions(tf.optsOnTangleVoting...))
	tf.gadget = New(tf.TestFramework.ValidatorSet(), tf.EvictionManager(), func(blockID models.BlockID) (block *models.Block, exists bool) {
		blk, exists := tf.TestFramework.BlockDAG().Block(blockID)
		if !exists {
			return nil, false
		}
		return blk.Block, true
	}, func(marker markers.Marker) (block *models.Block, exists bool) {
		blk, exists := tf.TestFramework.Booker().BlockFromMarker(marker)
		if !exists {
			return nil, false
		}
		return blk.Block.Block, true
	}, tf.optsGadget...)

	tf.TestFramework.OTV().Events.SequenceVoterAdded.Attach(event.NewClosure[*votes.SequenceVoterEvent](func(evt *votes.SequenceVoterEvent) {
		tf.gadget.update(evt.Voter, evt.NewMaxSupportedMarker, evt.PrevMaxSupportedMarker)
	}))

	tf.gadget.Events.BlockAccepted.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			tf.T.Logf("ACCEPTED: %s", metadata.ID())
		}

		atomic.AddUint32(&(tf.acceptedBlocks), 1)
	}))

	return
}

func (t *TestFramework) AssertBlockAccepted(blocksAccepted uint32) {
	assert.Equal(t.t, blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks), "expected %d blocks to be accepted but got %d", blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) ValidateAcceptedBlocks(expectedConflictIDs map[string]bool) {
	for blockID, blockExpectedAccepted := range expectedConflictIDs {
		actualBlockAccepted := t.gadget.IsBlockAccepted(t.Block(blockID).ID())
		assert.Equal(t.T, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (t *TestFramework) ValidateAcceptedMarker(expectedConflictIDs map[markers.Marker]bool) {
	for marker, markerExpectedAccepted := range expectedConflictIDs {
		actualMarkerAccepted := t.gadget.IsMarkerAccepted(marker)
		assert.Equal(t.T, markerExpectedAccepted, actualMarkerAccepted, "%s should be accepted=%t but is %t", marker, markerExpectedAccepted, actualMarkerAccepted)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithAcceptanceGadgetOptions(opts ...options.Option[AcceptanceGadget]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.optsGadget != nil {
			panic("OTV already set")
		}
		tf.optsGadget = opts
	}
}

func WithOnTangleVotingOptions(opts ...options.Option[otv.OnTangleVoting]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.optsOnTangleVoting != nil {
			panic("OTV already set")
		}
		tf.optsOnTangleVoting = opts
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
