package acceptance

import (
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/validator"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/models"
	"github.com/iotaledger/goshimmer/packages/protocol/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Gadget *Gadget

	test *testing.T

	acceptedBlocks    uint32
	conflictsAccepted uint32
	conflictsRejected uint32
	reorgCount        uint32

	optsGadget          []options.Option[Gadget]
	optsTangle          []options.Option[tangle.Tangle]
	optsValidatorSet    *validator.Set
	optsEvictionManager *eviction.Manager[models.BlockID]

	*tangle.TestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.optsEvictionManager == nil {
			t.optsEvictionManager = eviction.NewManager[models.BlockID](models.IsEmptyBlockID)
		}

		if t.optsValidatorSet == nil {
			t.optsValidatorSet = validator.NewSet()
		}

		t.TestFramework = tangle.NewTestFramework(
			test,
			tangle.WithTangleOptions(t.optsTangle...),
			tangle.WithValidatorSet(t.optsValidatorSet),
			tangle.WithEvictionManager(t.optsEvictionManager),
		)

		if t.Gadget == nil {
			t.Gadget = New(t.Tangle, t.optsGadget...)
		}

	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) setupEvents() {
	t.Gadget.Events.BlockAccepted.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("ACCEPTED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.acceptedBlocks), 1)
	}))

	t.Gadget.Events.Reorg.Hook(event.NewClosure(func(conflictID utxo.TransactionID) {
		if debug.GetEnabled() {
			t.test.Logf("REORG NEEDED: %s", conflictID)
		}
		atomic.AddUint32(&(t.reorgCount), 1)
	}))

	t.ConflictDAG().Events.ConflictAccepted.Hook(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT ACCEPTED: %s", event.ID)
		}
		atomic.AddUint32(&(t.conflictsAccepted), 1)
	}))

	t.ConflictDAG().Events.ConflictRejected.Hook(event.NewClosure(func(event *conflictdag.ConflictRejectedEvent[utxo.TransactionID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT REJECTED: %s", event.ID)
		}

		atomic.AddUint32(&(t.conflictsRejected), 1)
	}))
	return
}

func (t *TestFramework) AssertBlockAccepted(blocksAccepted uint32) {
	assert.Equal(t.test, blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks), "expected %d blocks to be accepted but got %d", blocksAccepted, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) AssertConflictsAccepted(conflictsAccepted uint32) {
	assert.Equal(t.test, conflictsAccepted, atomic.LoadUint32(&t.conflictsAccepted), "expected %d conflicts to be accepted but got %d", conflictsAccepted, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) AssertConflictsRejected(conflictsRejected uint32) {
	assert.Equal(t.test, conflictsRejected, atomic.LoadUint32(&t.conflictsRejected), "expected %d conflicts to be rejected but got %d", conflictsRejected, atomic.LoadUint32(&t.acceptedBlocks))
}

func (t *TestFramework) AssertReorgs(reorgCount uint32) {
	assert.Equal(t.test, reorgCount, atomic.LoadUint32(&t.reorgCount), "expected %d reorgs but got %d", reorgCount, atomic.LoadUint32(&t.reorgCount))
}

func (t *TestFramework) ValidateAcceptedBlocks(expectedConflictIDs map[string]bool) {
	for blockID, blockExpectedAccepted := range expectedConflictIDs {
		actualBlockAccepted := t.Gadget.IsBlockAccepted(t.Block(blockID).ID())
		assert.Equal(t.test, blockExpectedAccepted, actualBlockAccepted, "Block %s should be accepted=%t but is %t", blockID, blockExpectedAccepted, actualBlockAccepted)
	}
}

func (t *TestFramework) ValidateAcceptedMarker(expectedConflictIDs map[markers.Marker]bool) {
	for marker, markerExpectedAccepted := range expectedConflictIDs {
		actualMarkerAccepted := t.Gadget.IsMarkerAccepted(marker)
		assert.Equal(t.test, markerExpectedAccepted, actualMarkerAccepted, "%s should be accepted=%t but is %t", marker, markerExpectedAccepted, actualMarkerAccepted)
	}
}

func (t *TestFramework) ValidateConflictAcceptance(expectedConflictIDs map[string]confirmation.State) {
	for conflictIDAlias, conflictExpectedState := range expectedConflictIDs {
		actualMarkerAccepted := t.ConflictDAG().ConfirmationState(set.NewAdvancedSet(t.Transaction(conflictIDAlias).ID()))
		assert.Equal(t.test, conflictExpectedState, actualMarkerAccepted, "%s should be accepted=%t but is %t", conflictIDAlias, conflictExpectedState, actualMarkerAccepted)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithGadgetOptions(opts ...options.Option[Gadget]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsGadget = opts
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = opts
	}
}

func WithEvictionManager(evictionManager *eviction.Manager[models.BlockID]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsEvictionManager = evictionManager
	}
}

func WithValidatorSet(validatorSet *validator.Set) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsValidatorSet = validatorSet
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
