package booker

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/iotaledger/hive.go/core/debug"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
	"github.com/iotaledger/goshimmer/packages/core/tangleold/payload"
)

type TestFramework struct {
	Booker       *Booker
	genesisBlock *Block

	bookedBlocks          int32
	blockConflictsUpdated int32
	markerConflictsAdded  int32

	*tangle.TestFramework
	ledgerTf *ledger.TestFramework
}

func NewTestFramework(t *testing.T) (newTestFramework *TestFramework) {
	genesis := NewBlock(tangle.NewBlock(models.NewEmptyBlock(models.EmptyBlockID), tangle.WithSolid(true)), WithBooked(true), WithStructureDetails(markers.NewStructureDetails()))
	genesis.M.PayloadBytes = lo.PanicOnErr(payload.NewGenericDataPayload([]byte("")).Bytes())

	newTestFramework = &TestFramework{
		TestFramework: tangle.NewTestFramework(t),
		ledgerTf:      ledger.NewTestFramework(t),
		genesisBlock:  genesis,
	}
	newTestFramework.Booker = New(newTestFramework.Tangle, newTestFramework.ledgerTf.Ledger(), newTestFramework.rootBlockProvider)
	newTestFramework.Setup()
	return
}

func (t *TestFramework) Setup() {
	t.Booker.Events.BlockBooked.Hook(event.NewClosure(func(metadata *Block) {
		if debug.GetEnabled() {
			t.T.Logf("BOOKED: %s", metadata.ID())
		}
		atomic.AddInt32(&(t.bookedBlocks), 1)
	}))

	t.Booker.Events.BlockConflictUpdated.Hook(event.NewClosure(func(evt *BlockConflictUpdatedEvent) {
		if debug.GetEnabled() {
			t.T.Logf("BLOCK CONFLICT UPDATED: %s - %s", evt.Block.ID(), evt.ConflictID)
		}
		atomic.AddInt32(&(t.blockConflictsUpdated), 1)
	}))

	t.Booker.Events.MarkerConflictAdded.Hook(event.NewClosure(func(evt *MarkerConflictAddedEvent) {
		if debug.GetEnabled() {
			t.T.Logf("BLOCK CONFLICT UPDATED: %v - %v", evt.Marker, evt.NewConflictID)
		}
		atomic.AddInt32(&(t.markerConflictsAdded), 1)
	}))

	t.Booker.Events.Error.Hook(event.NewClosure(func(err error) {
		t.T.Logf("ERROR: %s", err)
	}))
}

// Block retrieves the Blocks that is associated with the given alias.
func (t *TestFramework) Block(alias string) (block *Block) {
	innerBlock := t.TestFramework.Block(alias)
	block, ok := t.Booker.block(innerBlock.ID())
	if !ok {
		panic(fmt.Sprintf("Block alias %s not registered", alias))
	}

	return block
}

func (t *TestFramework) AssertBlock(alias string, callback func(block *Block)) {
	block, exists := t.Booker.Block(t.Block(alias).ID())
	require.True(t.T, exists, "Block %s not found", alias)
	callback(block)
}

func (t *TestFramework) AssertSolid(expectedValues map[string]bool) {
	for alias, isSolid := range expectedValues {
		t.AssertBlock(alias, func(block *Block) {
			assert.Equal(t.T, isSolid, block.IsSolid(), "block %s has incorrect solid flag", alias)
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
		_, retrievedConflictIDs := t.Booker.blockBookingDetails(t.Block(blockID))
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
			currentMarker := expectedMarkersOfBlock.Marker()

			mappedBlockIDOfMarker, exists := t.Booker.markerManager.BlockFromMarker(currentMarker)
			assert.True(t.T, exists, "Marker %s is not mapped to any block", currentMarker)

			if mappedBlockIDOfMarker.ID() != block.ID() {
				continue
			}

			// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
			if currentMarker.SequenceID() == 0 && currentMarker.Index() == 0 {
				return
			}

			if assert.True(t.T, block.StructureDetails().IsPastMarker(), "Block with %s should be PastMarker", block.ID()) {
				assert.True(t.T, block.StructureDetails().PastMarkers().Marker() == currentMarker, "PastMarker of %s is wrong.\n"+
					"Expected: %+v\nActual: %+v", block.ID(), currentMarker, block.StructureDetails().PastMarkers().Marker())
			}
		}
	}
}

func (t *TestFramework) checkNormalizedConflictIDsContained(expectedContainedConflictIDs map[string]utxo.TransactionIDs) {
	for blockAlias, blockExpectedConflictIDs := range expectedContainedConflictIDs {
		_, retrievedConflictIDs := t.Booker.blockBookingDetails(t.Block(blockAlias))

		normalizedRetrievedConflictIDs := retrievedConflictIDs.Clone()
		for it := retrievedConflictIDs.Iterator(); it.HasNext(); {
			t.ledgerTf.Ledger().ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedRetrievedConflictIDs.DeleteAll(b.Parents())
			})
		}

		normalizedExpectedConflictIDs := blockExpectedConflictIDs.Clone()
		for it := blockExpectedConflictIDs.Iterator(); it.HasNext(); {
			t.ledgerTf.Ledger().ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
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

// rootBlockProvider is a default function that determines whether a block is a root of the Tangle.
func (t *TestFramework) rootBlockProvider(blockID models.BlockID) (block *Block) {
	if blockID != t.genesisBlock.ID() {
		return
	}

	return t.genesisBlock
}
