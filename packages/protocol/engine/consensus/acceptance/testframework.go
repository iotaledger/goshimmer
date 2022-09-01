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
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework //////////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Gadget              *Gadget
	TangleTestFramework *tangle.TestFramework

	test              *testing.T
	acceptedBlocks    uint32
	conflictsAccepted uint32
	conflictsRejected uint32
	reorgCount        uint32

	optsGadgetOptions   []options.Option[Gadget]
	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsEvictionManager *eviction.Manager[models.BlockID]
	optsValidatorSet    *validator.Set
	optsTangle          *tangle.Tangle
	optsTangleOptions   []options.Option[tangle.Tangle]
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (t *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		if t.Gadget == nil {
			if t.optsTangle == nil {
				if t.optsLedger == nil {
					t.optsLedger = ledger.New(t.optsLedgerOptions...)
				}

				if t.optsEvictionManager == nil {
					t.optsEvictionManager = eviction.NewManager[models.BlockID](models.IsEmptyBlockID)
				}

				if t.optsValidatorSet == nil {
					t.optsValidatorSet = validator.NewSet()
				}

				t.optsTangle = tangle.New(t.optsLedger, t.optsEvictionManager, t.optsValidatorSet, t.optsTangleOptions...)
			}

			t.Gadget = New(t.optsTangle, t.optsGadgetOptions...)
		}

		t.TangleTestFramework = tangle.NewTestFramework(test, tangle.WithTangle(t.optsTangle))
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

	t.TangleTestFramework.ConflictDAG().Events.ConflictAccepted.Hook(event.NewClosure(func(event *conflictdag.ConflictAcceptedEvent[utxo.TransactionID]) {
		if debug.GetEnabled() {
			t.test.Logf("CONFLICT ACCEPTED: %s", event.ID)
		}
		atomic.AddUint32(&(t.conflictsAccepted), 1)
	}))

	t.TangleTestFramework.ConflictDAG().Events.ConflictRejected.Hook(event.NewClosure(func(event *conflictdag.ConflictRejectedEvent[utxo.TransactionID]) {
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
		actualBlockAccepted := t.Gadget.IsBlockAccepted(t.TangleTestFramework.Block(blockID).ID())
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
		actualMarkerAccepted := t.TangleTestFramework.ConflictDAG().ConfirmationState(set.NewAdvancedSet(t.TangleTestFramework.Transaction(conflictIDAlias).ID()))
		assert.Equal(t.test, conflictExpectedState, actualMarkerAccepted, "%s should be accepted=%t but is %t", conflictIDAlias, conflictExpectedState, actualMarkerAccepted)
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithGadget(gadget *Gadget) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.Gadget = gadget
	}
}

func WithGadgetOptions(opts ...options.Option[Gadget]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsGadgetOptions = opts
	}
}

func WithTangle(tangle *tangle.Tangle) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangle = tangle
	}
}

func WithTangleOptions(opts ...options.Option[tangle.Tangle]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsTangleOptions = opts
	}
}

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsLedger = ledger
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		tf.optsLedgerOptions = opts
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
