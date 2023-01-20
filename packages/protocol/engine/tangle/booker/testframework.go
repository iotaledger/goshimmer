package booker

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	Booker *Booker

	test                  *testing.T
	bookedBlocks          int32
	blockConflictsUpdated int32
	markerConflictsAdded  int32

	optsBlockDAG        *blockdag.BlockDAG
	optsBlockDAGOptions []options.Option[blockdag.BlockDAG]
	optsLedger          *ledger.Ledger
	optsLedgerOptions   []options.Option[ledger.Ledger]
	optsBookerOptions   []options.Option[Booker]

	*LedgerTestFramework
	*BlockDAGTestFramework
}

func NewTestFramework(test *testing.T, opts ...options.Option[TestFramework]) (newTestFramework *TestFramework) {
	return options.Apply(&TestFramework{
		test: test,
	}, opts, func(t *TestFramework) {
		t.BlockDAGTestFramework = blockdag.NewTestFramework(test, lo.Cond(t.optsBlockDAG != nil, blockdag.WithBlockDAG(t.optsBlockDAG), blockdag.WithBlockDAGOptions(t.optsBlockDAGOptions...)))
		t.LedgerTestFramework = ledger.NewTestFramework(test, lo.Cond(t.optsLedger != nil, ledger.WithLedger(t.optsLedger), ledger.WithLedgerOptions(t.optsLedgerOptions...)))

		if t.Booker == nil {
			t.Booker = New(t.BlockDAG, t.Ledger, t.optsBookerOptions...)
		}
	}, (*TestFramework).setupEvents)
}

func (t *TestFramework) SequenceManager() (sequenceManager *markers.SequenceManager) {
	return t.Booker.markerManager.SequenceManager
}

func (t *TestFramework) PreventNewMarkers(prevent bool) {
	callback := func(markers.SequenceID, markers.Index) bool {
		return !prevent
	}
	t.Booker.markerManager.SequenceManager.SetIncreaseIndexCallback(callback)
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	block, ok := t.Booker.block(t.BlockDAGTestFramework.Block(alias).ID())
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return block
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Booker.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertBooked(expectedValues map[string]bool) {
	for alias, isBooked := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.test, isBooked, block.IsBooked(), "block %s has incorrect booked flag", alias)
		})
	}
}

func (t *TestFramework) AssertBookedCount(bookedCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, bookedCount, atomic.LoadInt32(&(t.bookedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMarkerConflictsAddCount(markerConflictsAddCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, markerConflictsAddCount, atomic.LoadInt32(&(t.markerConflictsAdded)), msgAndArgs...)
}

func (t *TestFramework) AssertBlockConflictsUpdateCount(blockConflictsUpdateCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.test, blockConflictsUpdateCount, atomic.LoadInt32(&(t.blockConflictsUpdated)), msgAndArgs...)
}

func (t *TestFramework) setupEvents() {
	t.Booker.Events.BlockBooked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.test.Logf("BOOKED: %v, %v", metadata.ID(), metadata.StructureDetails().PastMarkers())
		}

		atomic.AddInt32(&(t.bookedBlocks), 1)
	}))

	t.Booker.Events.BlockConflictAdded.Hook(event.NewClosure(func(evt *BlockConflictAddedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK CONFLICT UPDATED: %s - %s", evt.Block.ID(), evt.ConflictID)
		}

		atomic.AddInt32(&(t.blockConflictsUpdated), 1)
	}))

	t.Booker.Events.MarkerConflictAdded.Hook(event.NewClosure(func(evt *MarkerConflictAddedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("MARKER CONFLICT UPDATED: %v - %v", evt.Marker, evt.ConflictID)
		}

		atomic.AddInt32(&(t.markerConflictsAdded), 1)
	}))

	t.Booker.Events.Error.Hook(event.NewClosure(func(err error) {
		t.test.Logf("ERROR: %s", err)
	}))
}

func (t *TestFramework) checkConflictIDs(expectedConflictIDs map[string]utxo.TransactionIDs) {
	for blockID, blockExpectedConflictIDs := range expectedConflictIDs {
		_, retrievedConflictIDs := t.Booker.blockBookingDetails(t.Block(blockID))
		require.True(t.test, blockExpectedConflictIDs.Equal(retrievedConflictIDs), "ConflictID of %s should be %s but is %s", blockID, blockExpectedConflictIDs, retrievedConflictIDs)
	}
}

func (t *TestFramework) CheckMarkers(expectedMarkers map[string]*markers.Markers) {
	for blockAlias, expectedMarkersOfBlock := range expectedMarkers {
		block := t.Block(blockAlias)
		require.True(t.test, expectedMarkersOfBlock.Equals(block.StructureDetails().PastMarkers()), "Markers of %s are wrong.\n"+
			"Expected: %+v\nActual: %+v", blockAlias, expectedMarkersOfBlock, block.StructureDetails().PastMarkers())

		// if we have only a single marker - check if the marker is mapped to this block (or its inherited past marker)
		if expectedMarkersOfBlock.Size() == 1 {
			expectedMarker := expectedMarkersOfBlock.Marker()

			// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
			if expectedMarker.SequenceID() == 0 && expectedMarker.Index() == 0 {
				continue
			}

			mappedBlockIDOfMarker, exists := t.Booker.markerManager.BlockFromMarker(expectedMarker)
			require.True(t.test, exists, "Marker %s is not mapped to any block", expectedMarker)

			if !block.StructureDetails().IsPastMarker() {
				continue
			}

			require.Equal(t.test, block.ID(), mappedBlockIDOfMarker.ID(), "Block with %s should be past marker %s", block.ID(), expectedMarker)
			require.True(t.test, block.StructureDetails().PastMarkers().Marker() == expectedMarker, "PastMarker of %s is wrong.\n"+
				"Expected: %+v\nActual: %+v", block.ID(), expectedMarker, block.StructureDetails().PastMarkers().Marker())
		}
	}
}

func (t *TestFramework) checkNormalizedConflictIDsContained(expectedContainedConflictIDs map[string]utxo.TransactionIDs) {
	for blockAlias, blockExpectedConflictIDs := range expectedContainedConflictIDs {
		_, retrievedConflictIDs := t.Booker.blockBookingDetails(t.Block(blockAlias))

		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.ConflictDAG.Conflict(it.Next())
			require.True(t.test, exists, "conflict %s does not exist", conflict.ID())
			normalizedRetrievedConflictIDs.DeleteAll(conflict.Parents())
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.ConflictDAG.Conflict(it.Next())
			require.True(t.test, exists, "conflict %s does not exist", conflict.ID())
			normalizedExpectedConflictIDs.DeleteAll(conflict.Parents())
		}

		require.True(t.test, normalizedExpectedConflictIDs.Intersect(normalizedRetrievedConflictIDs).Size() == normalizedExpectedConflictIDs.Size(), "ConflictID of %s should be %s but is %s", blockAlias, normalizedExpectedConflictIDs, normalizedRetrievedConflictIDs)
	}
}

func (t *TestFramework) checkBlockMetadataDiffConflictIDs(expectedDiffConflictIDs map[string][]utxo.TransactionIDs) {
	for blockAlias, expectedDiffConflictID := range expectedDiffConflictIDs {
		block := t.Block(blockAlias)
		require.True(t.test, expectedDiffConflictID[0].Equal(block.AddedConflictIDs()), "AddConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[0], block.AddedConflictIDs())
		require.True(t.test, expectedDiffConflictID[1].Equal(block.SubtractedConflictIDs()), "SubtractedConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[1], block.SubtractedConflictIDs())
	}
}

type BlockDAGTestFramework = blockdag.TestFramework

type LedgerTestFramework = ledger.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBlockDAGOptions(opts ...options.Option[blockdag.BlockDAG]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAGOptions = opts
	}
}

func WithBlockDAG(blockDAG *blockdag.BlockDAG) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBlockDAG = blockDAG
	}
}

func WithLedgerOptions(opts ...options.Option[ledger.Ledger]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedgerOptions = opts
	}
}

func WithLedger(ledger *ledger.Ledger) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsLedger = ledger
	}
}

func WithBookerOptions(opts ...options.Option[Booker]) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.optsBookerOptions = opts
	}
}

func WithBooker(booker *Booker) options.Option[TestFramework] {
	return func(t *TestFramework) {
		t.Booker = booker
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
