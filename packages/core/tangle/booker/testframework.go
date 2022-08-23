package booker

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/eviction"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// region TestFramework ////////////////////////////////////////////////////////////////////////////////////////////////

type TestFramework struct {
	evictionManager       *eviction.Manager[models.BlockID]
	booker                *Booker
	bookedBlocks          int32
	blockConflictsUpdated int32
	markerConflictsAdded  int32
	optsBooker            []options.Option[Booker]

	*tangleTestFramework
	*ledgerTestFramework
}

func NewTestFramework(t *testing.T, opts ...options.Option[TestFramework]) (testFramework *TestFramework) {
	testFramework = options.Apply(new(TestFramework), opts)
	testFramework.ledgerTestFramework = ledger.NewTestFramework(t, ledger.WithLedger(testFramework.Booker().Ledger))
	testFramework.tangleTestFramework = blockdag.NewTestFramework(t, blockdag.WithTangle(testFramework.Booker().BlockDAG), blockdag.WithEvictionManager(testFramework.EvictionManager()))

	testFramework.Booker().Events.BlockBooked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			testFramework.T.Logf("BOOKED: %s", metadata.ID())
		}

		atomic.AddInt32(&(testFramework.bookedBlocks), 1)
	}))

	testFramework.Booker().Events.BlockConflictAdded.Hook(event.NewClosure(func(evt *BlockConflictAddedEvent) {
		if debug.GetEnabled() {
			testFramework.T.Logf("BLOCK CONFLICT UPDATED: %s - %s", evt.Block.ID(), evt.ConflictID)
		}

		atomic.AddInt32(&(testFramework.blockConflictsUpdated), 1)
	}))

	testFramework.Booker().Events.MarkerConflictAdded.Hook(event.NewClosure(func(evt *MarkerConflictAddedEvent) {
		if debug.GetEnabled() {
			testFramework.T.Logf("MARKER CONFLICT UPDATED: %v - %v", evt.Marker, evt.ConflictID)
		}

		atomic.AddInt32(&(testFramework.markerConflictsAdded), 1)
	}))

	testFramework.Booker().Events.Error.Hook(event.NewClosure(func(err error) {
		testFramework.T.Logf("ERROR: %s", err)
	}))

	return
}

func (t *TestFramework) Booker() (booker *Booker) {
	if t.booker == nil {
		t.booker = New(t.EvictionManager(), t.optsBooker...)
	}

	return t.booker
}

func (t *TestFramework) SequenceManager() (sequenceManager *markers.SequenceManager) {
	return t.Booker().markerManager.sequenceManager
}

func (t *TestFramework) EvictionManager() *eviction.Manager[models.BlockID] {
	if t.evictionManager == nil {
		if t.booker != nil {
			t.evictionManager = t.booker.evictionManager.Manager
		} else {
			t.evictionManager = eviction.NewManager(models.IsEmptyBlockID)
		}
	}

	return t.evictionManager
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	block, ok := t.Booker().block(t.tangleTestFramework.Block(alias).ID())
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return block
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Booker().Block(t.Block(alias).ID())
	require.True(t.T, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertBooked(expectedValues map[string]bool) {
	for alias, isBooked := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, isBooked, block.IsBooked(), "block %s has incorrect booked flag", alias)
		})
	}
}

func (t *TestFramework) AssertBookedCount(bookedCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, bookedCount, atomic.LoadInt32(&(t.bookedBlocks)), msgAndArgs...)
}

func (t *TestFramework) AssertMarkerConflictsAddCount(markerConflictsAddCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, markerConflictsAddCount, atomic.LoadInt32(&(t.markerConflictsAdded)), msgAndArgs...)
}

func (t *TestFramework) AssertBlockConflictsUpdateCount(blockConflictsUpdateCount int32, msgAndArgs ...interface{}) {
	assert.EqualValues(t.T, blockConflictsUpdateCount, atomic.LoadInt32(&(t.blockConflictsUpdated)), msgAndArgs...)
}

func (t *TestFramework) checkConflictIDs(expectedConflictIDs map[string]utxo.TransactionIDs) {
	for blockID, blockExpectedConflictIDs := range expectedConflictIDs {
		_, retrievedConflictIDs := t.Booker().blockBookingDetails(t.Block(blockID))
		assert.True(t.T, blockExpectedConflictIDs.Equal(retrievedConflictIDs), "ConflictID of %s should be %s but is %s", blockID, blockExpectedConflictIDs, retrievedConflictIDs)
	}
}

func (t *TestFramework) checkMarkers(expectedMarkers map[string]*markers.Markers) {
	for blockAlias, expectedMarkersOfBlock := range expectedMarkers {
		block := t.Block(blockAlias)
		assert.True(t.T, expectedMarkersOfBlock.Equals(block.StructureDetails().PastMarkers()), "Markers of %s are wrong.\n"+
			"Expected: %+v\nActual: %+v", blockAlias, expectedMarkersOfBlock, block.StructureDetails().PastMarkers())

		// if we have only a single marker - check if the marker is mapped to this block (or its inherited past marker)
		if expectedMarkersOfBlock.Size() == 1 {
			expectedMarker := expectedMarkersOfBlock.Marker()

			// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
			if expectedMarker.SequenceID() == 0 && expectedMarker.Index() == 0 {
				continue
			}

			mappedBlockIDOfMarker, exists := t.Booker().markerManager.BlockFromMarker(expectedMarker)
			assert.True(t.T, exists, "Marker %s is not mapped to any block", expectedMarker)

			if !block.StructureDetails().IsPastMarker() {
				continue
			}

			assert.Equal(t.T, block.ID(), mappedBlockIDOfMarker.ID(), "Block with %s should be past marker %s", block.ID(), expectedMarker)
			assert.True(t.T, block.StructureDetails().PastMarkers().Marker() == expectedMarker, "PastMarker of %s is wrong.\n"+
				"Expected: %+v\nActual: %+v", block.ID(), expectedMarker, block.StructureDetails().PastMarkers().Marker())
		}
	}
}

func (t *TestFramework) checkNormalizedConflictIDsContained(expectedContainedConflictIDs map[string]utxo.TransactionIDs) {
	for blockAlias, blockExpectedConflictIDs := range expectedContainedConflictIDs {
		_, retrievedConflictIDs := t.Booker().blockBookingDetails(t.Block(blockAlias))

		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
			t.Ledger().ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedRetrievedConflictIDs.DeleteAll(b.Parents())
			})
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			t.Ledger().ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedExpectedConflictIDs.DeleteAll(b.Parents())
			})
		}

		assert.True(t.T, normalizedExpectedConflictIDs.Intersect(normalizedRetrievedConflictIDs).Size() == normalizedExpectedConflictIDs.Size(), "ConflictID of %s should be %s but is %s", blockAlias, normalizedExpectedConflictIDs, normalizedRetrievedConflictIDs)
	}
}

func (t *TestFramework) checkBlockMetadataDiffConflictIDs(expectedDiffConflictIDs map[string][]utxo.TransactionIDs) {
	for blockAlias, expectedDiffConflictID := range expectedDiffConflictIDs {
		block := t.Block(blockAlias)
		assert.True(t.T, expectedDiffConflictID[0].Equal(block.AddedConflictIDs()), "AddConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[0], block.AddedConflictIDs())
		assert.True(t.T, expectedDiffConflictID[1].Equal(block.SubtractedConflictIDs()), "SubtractedConflictIDs of %s should be %s but is %s in the Metadata", blockAlias, expectedDiffConflictID[1], block.SubtractedConflictIDs())
	}
}

type tangleTestFramework = blockdag.TestFramework

type ledgerTestFramework = ledger.TestFramework

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

func WithBookerOptions(opts ...options.Option[Booker]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.booker != nil {
			panic("Booker already set")
		}
		tf.optsBooker = opts
	}
}

func WithBooker(booker *Booker) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.optsBooker != nil {
			panic("Booker options already set")
		}
		tf.booker = booker
	}
}

func WithEvictionManager(evictionManager *eviction.Manager[models.BlockID]) options.Option[TestFramework] {
	return func(tf *TestFramework) {
		if tf.evictionManager != nil {
			panic("Eviction manager already set")
		}
		tf.evictionManager = evictionManager
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
