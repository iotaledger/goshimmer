package booker

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/votes"
	"github.com/iotaledger/goshimmer/packages/core/votes/sequencetracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/runtime/debug"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

type TestFramework struct {
	Test            *testing.T
	Workers         *workerpool.Group
	Instance        Booker
	Ledger          *mempool.TestFramework
	BlockDAG        *blockdag.TestFramework
	ConflictDAG     *conflictdag.TestFramework
	VirtualVoting   *VirtualVotingTestFramework
	SequenceTracker *sequencetracker.TestFramework[BlockVotePower]
	Votes           *votes.TestFramework

	bookedBlocks          int32
	blockConflictsUpdated int32
	markerConflictsAdded  int32
	trackedBlocks         uint32
}

func NewTestFramework(test *testing.T, workers *workerpool.Group, instance Booker, blockDAG blockdag.BlockDAG, memPool mempool.MemPool, validators *sybilprotection.WeightedSet, slotTimeProviderFunc func() *slot.TimeProvider) *TestFramework {
	t := &TestFramework{
		Test:          test,
		Workers:       workers,
		Instance:      instance,
		BlockDAG:      blockdag.NewTestFramework(test, workers.CreateGroup("BlockDAG"), blockDAG, slotTimeProviderFunc),
		ConflictDAG:   conflictdag.NewTestFramework(test, memPool.ConflictDAG()),
		Ledger:        mempool.NewTestFramework(test, memPool),
		VirtualVoting: NewVirtualVotingTestFramework(test, instance.VirtualVoting(), memPool, validators),
	}

	t.Votes = votes.NewTestFramework(test, validators)

	t.SequenceTracker = sequencetracker.NewTestFramework(test,
		t.Votes,
		t.Instance.SequenceTracker(),
		t.Instance.SequenceManager(),
	)

	t.setupEvents()
	return t
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	block, ok := t.Instance.Block(t.BlockDAG.Block(alias).ID())
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return block
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Instance.Block(t.Block(alias).ID())
	require.True(t.Test, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertBooked(expectedValues map[string]bool) {
	for alias, isBooked := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			require.Equal(t.Test, isBooked, block.IsBooked(), "block %s has incorrect booked flag", alias)
		})
	}
}

func (t *TestFramework) AssertBookedCount(bookedCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, bookedCount, atomic.LoadInt32(&(t.bookedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMarkerConflictsAddCount(markerConflictsAddCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, markerConflictsAddCount, atomic.LoadInt32(&(t.markerConflictsAdded)), msgAndArgs...)
}

func (t *TestFramework) AssertBlockConflictsUpdateCount(blockConflictsUpdateCount int32, msgAndArgs ...interface{}) {
	require.EqualValues(t.Test, blockConflictsUpdateCount, atomic.LoadInt32(&(t.blockConflictsUpdated)), msgAndArgs...)
}

func (t *TestFramework) setupEvents() {
	t.Instance.Events().BlockBooked.Hook(func(evt *BlockBookedEvent) {
		if debug.GetEnabled() {
			t.Test.Logf("BOOKED: %v, %v", evt.Block.ID(), evt.Block.StructureDetails().PastMarkers())
		}

		atomic.AddInt32(&(t.bookedBlocks), 1)
	})

	t.Instance.Events().BlockConflictAdded.Hook(func(evt *BlockConflictAddedEvent) {
		if debug.GetEnabled() {
			t.Test.Logf("BLOCK CONFLICT UPDATED: %s - %s", evt.Block.ID(), evt.ConflictID)
		}

		atomic.AddInt32(&(t.blockConflictsUpdated), 1)
	})

	t.Instance.Events().MarkerConflictAdded.Hook(func(evt *MarkerConflictAddedEvent) {
		if debug.GetEnabled() {
			t.Test.Logf("MARKER CONFLICT UPDATED: %v - %v", evt.Marker, evt.ConflictID)
		}

		atomic.AddInt32(&(t.markerConflictsAdded), 1)
	})

	t.Instance.Events().Error.Hook(func(err error) {
		t.Test.Logf("ERROR: %s", err)
	})

	t.Instance.Events().BlockTracked.Hook(func(metadata *Block) {
		if debug.GetEnabled() {
			t.Test.Logf("TRACKED: %s", metadata.ID())
		}

		atomic.AddUint32(&(t.trackedBlocks), 1)
	})
}

func (t *TestFramework) CheckConflictIDs(expectedConflictIDs map[string]utxo.TransactionIDs) {
	for blockID, blockExpectedConflictIDs := range expectedConflictIDs {
		block := t.Block(blockID)
		require.NotNil(t.Test, block)
		require.NotNil(t.Test, block.StructureDetails)
		_, retrievedConflictIDs := t.Instance.BlockBookingDetails(block)
		require.True(t.Test, blockExpectedConflictIDs.Equal(retrievedConflictIDs), "ConflictID of %s should be %s but is %s", blockID, blockExpectedConflictIDs, retrievedConflictIDs)
	}
}

func (t *TestFramework) CheckMarkers(expectedMarkers map[string]*markers.Markers) {
	for blockAlias, expectedMarkersOfBlock := range expectedMarkers {
		block := t.Block(blockAlias)
		require.True(t.Test, expectedMarkersOfBlock.Equals(block.StructureDetails().PastMarkers()), "Markers of %s are wrong.\n"+
			"Expected: %+v\nActual: %+v", blockAlias, expectedMarkersOfBlock, block.StructureDetails().PastMarkers())

		// if we have only a single marker - check if the marker is mapped to this block (or its inherited past marker)
		if expectedMarkersOfBlock.Size() == 1 {
			expectedMarker := expectedMarkersOfBlock.Marker()

			// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
			if expectedMarker.SequenceID() == 0 && expectedMarker.Index() == 0 {
				continue
			}

			mappedBlockIDOfMarker, exists := t.Instance.BlockFromMarker(expectedMarker)
			require.True(t.Test, exists, "Marker %s is not mapped to any block", expectedMarker)

			if !block.StructureDetails().IsPastMarker() {
				continue
			}

			require.Equal(t.Test, block.ID(), mappedBlockIDOfMarker.ID(), "Block with %s should be past marker %s", block.ID(), expectedMarker)
			require.True(t.Test, block.StructureDetails().PastMarkers().Marker() == expectedMarker, "PastMarker of %s is wrong.\n"+
				"Expected: %+v\nActual: %+v", block.ID(), expectedMarker, block.StructureDetails().PastMarkers().Marker())
		}
	}
}

func (t *TestFramework) CheckNormalizedConflictIDsContained(expectedContainedConflictIDs map[string]utxo.TransactionIDs) {
	for blockAlias, blockExpectedConflictIDs := range expectedContainedConflictIDs {
		_, retrievedConflictIDs := t.Instance.BlockBookingDetails(t.Block(blockAlias))

		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.Instance.ConflictDAG().Conflict(it.Next())
			require.True(t.Test, exists, "conflict %s does not exist", conflict.ID())
			normalizedRetrievedConflictIDs.DeleteAll(conflict.Parents())
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			conflict, exists := t.Ledger.Instance.ConflictDAG().Conflict(it.Next())
			require.True(t.Test, exists, "conflict %s does not exist", conflict.ID())
			normalizedExpectedConflictIDs.DeleteAll(conflict.Parents())
		}

		require.True(t.Test, normalizedExpectedConflictIDs.Intersect(normalizedRetrievedConflictIDs).Size() == normalizedExpectedConflictIDs.Size(), "ConflictID of %s should be %s but is %s", blockAlias, normalizedExpectedConflictIDs, normalizedRetrievedConflictIDs)
	}
}

func (t *TestFramework) CheckBlockMetadataDiffConflictIDs(expectedDiffConflictIDs map[string][]utxo.TransactionIDs) {
	for blockAlias, expectedDiffConflictID := range expectedDiffConflictIDs {
		block := t.Block(blockAlias)
		require.True(t.Test, expectedDiffConflictID[0].Equal(block.AddedConflictIDs()), "AddConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[0], block.AddedConflictIDs())
		require.True(t.Test, expectedDiffConflictID[1].Equal(block.SubtractedConflictIDs()), "SubtractedConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[1], block.SubtractedConflictIDs())
	}
}

func (t *TestFramework) ValidateMarkerVoters(expectedVoters map[markers.Marker]*advancedset.AdvancedSet[identity.ID]) {
	for marker, expectedVotersOfMarker := range expectedVoters {
		voters := t.Instance.SequenceTracker().Voters(marker)

		assert.True(t.Test, expectedVotersOfMarker.Equal(voters), "marker %s expected %d voters but got %d", marker, expectedVotersOfMarker.Size(), voters.Size())
	}
}

func (t *TestFramework) AssertBlockTracked(blocksTracked uint32) {
	assert.Equal(t.Test, blocksTracked, atomic.LoadUint32(&t.trackedBlocks), "expected %d blocks to be tracked but got %d", blocksTracked, atomic.LoadUint32(&t.trackedBlocks))
}
