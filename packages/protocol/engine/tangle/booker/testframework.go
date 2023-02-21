package booker

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/eviction"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	test          *testing.T
	Workers       *workerpool.Group
	Instance      *Booker
	Ledger        *ledger.TestFramework
	BlockDAG      *blockdag.TestFramework
	ConflictDAG   *conflictdag.TestFramework
	VirtualVoting *virtualvoting.TestFramework

	bookedBlocks          int32
	blockConflictsUpdated int32
	markerConflictsAdded  int32
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, instance *Booker) *TestFramework {
	t := &TestFramework{
		test:          test,
		Workers:       workers,
		Instance:      instance,
		BlockDAG:      blockdag.NewTestFramework(test, workers.CreateGroup("BlockDAG"), instance.BlockDAG),
		ConflictDAG:   conflictdag.NewTestFramework(test, instance.Ledger.ConflictDAG),
		Ledger:        ledger.NewTestFramework(test, instance.Ledger),
		VirtualVoting: virtualvoting.NewTestFramework(test, workers.CreateGroup("VirtualVoting"), instance.VirtualVoting),
	}

	t.setupEvents()
	return t
}

func NewDefaultTestFramework(t *testing.T, workers *workerpool.Group, optsBooker ...options.Option[Booker]) *TestFramework {
	storageInstance := blockdag.NewTestStorage(t, workers)

	return NewTestFramework(t, workers, New(
		workers.CreateGroup("Booker"),
		blockdag.NewTestBlockDAG(t, workers, eviction.NewState(storageInstance), blockdag.DefaultCommitmentFunc),
		ledger.NewTestLedger(t, workers.CreateGroup("Ledger")),
		sybilprotection.NewWeightedSet(sybilprotection.NewWeights(mapdb.NewMapDB())),
		optsBooker...,
	))
}

func (t *TestFramework) SequenceManager() (sequenceManager *markers.SequenceManager) {
	return t.Instance.markerManager.SequenceManager
}

func (t *TestFramework) PreventNewMarkers(prevent bool) {
	callback := func(markers.SequenceID, markers.Index) bool {
		return !prevent
	}
	t.Instance.markerManager.SequenceManager.SetIncreaseIndexCallback(callback)
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *virtualvoting.Block) {
	block, ok := t.Instance.Block(t.BlockDAG.Block(alias).ID())
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return block
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *virtualvoting.Block)) {
	block, exists := t.Instance.Block(t.Block(alias).ID())
	require.True(t.test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertBooked(expectedValues map[string]bool) {
	for alias, isBooked := range expectedValues {
		t.AssertBlock(alias, func(block *virtualvoting.Block) {
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
	t.Instance.Events.BlockBooked.Hook(func(evt *BlockBookedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("BOOKED: %v, %v", evt.Block.ID(), evt.Block.StructureDetails().PastMarkers())
		}

		atomic.AddInt32(&(t.bookedBlocks), 1)
	})

	t.Instance.Events.BlockConflictAdded.Hook(func(evt *BlockConflictAddedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("BLOCK CONFLICT UPDATED: %s - %s", evt.Block.ID(), evt.ConflictID)
		}

		atomic.AddInt32(&(t.blockConflictsUpdated), 1)
	})

	t.Instance.Events.MarkerConflictAdded.Hook(func(evt *MarkerConflictAddedEvent) {
		if debug.GetEnabled() {
			t.test.Logf("MARKER CONFLICT UPDATED: %v - %v", evt.Marker, evt.ConflictID)
		}

		atomic.AddInt32(&(t.markerConflictsAdded), 1)
	})

	t.Instance.Events.Error.Hook(func(err error) {
		t.test.Logf("ERROR: %s", err)
	})
}

func (t *TestFramework) checkConflictIDs(expectedConflictIDs map[string]utxo.TransactionIDs) {
	for blockID, blockExpectedConflictIDs := range expectedConflictIDs {
		block := t.Block(blockID)
		require.NotNil(t.test, block)
		require.NotNil(t.test, block.StructureDetails)
		_, retrievedConflictIDs := t.Instance.blockBookingDetails(block)
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

			mappedBlockIDOfMarker, exists := t.Instance.markerManager.BlockFromMarker(expectedMarker)
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
		_, retrievedConflictIDs := t.Instance.blockBookingDetails(t.Block(blockAlias))

		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.Instance.ConflictDAG.Conflict(it.Next())
			require.True(t.test, exists, "conflict %s does not exist", conflict.ID())
			normalizedRetrievedConflictIDs.DeleteAll(conflict.Parents())
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.Instance.ConflictDAG.Conflict(it.Next())
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
