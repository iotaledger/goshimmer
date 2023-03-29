package markermanager

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag/inmemoryblockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

// We create slotCount blocks, each in a different slot and with a different marker, then we prune the markerManager and expect the mapping to be pruned accordingly.
func Test_PruneMarkerBlockMapping(t *testing.T) {
	const slotCount = 100

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	workers := workerpool.NewGroup(t.Name())
	tf := inmemoryblockdag.NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	tf.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().Events.SlotEvicted.Hook(markerManager.Evict)

	// create a helper function that creates the blocks
	createNewBlock := func(idx slot.Index, prefix string) (block *blockdag.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			return blockdag.NewBlock(tf.CreateBlock(
				alias,
				models.WithIssuingTime(tf.SlotTimeProvider().GenesisTime()),
			)), alias
		}
		return blockdag.NewBlock(tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(tf.SlotTimeProvider().StartTime(idx)),
		)), alias
	}

	markerBlockMapping := make(map[markers.Marker]*blockdag.Block, slotCount)
	for i := slot.Index(1); i <= slotCount; i++ {
		blk, _ := createNewBlock(i, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	assert.Equal(t, slotCount, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to have %d elements", slotCount)
	assert.Equal(t, slotCount, markerManager.markerBlockMappingEviction.Size(), "expected the marker block pruning map to have %d elements", slotCount)
	assert.Equal(t, 1, markerManager.sequenceMarkersMapping.Size(), "expected the sequence marker tree map to be have 1 element")

	markerIndexTree, exists := markerManager.sequenceMarkersMapping.Get(1)
	assert.True(t, exists, "expected the sequence marker tree map to be contain tree map for SequenceID(1)")
	assert.Equal(t, slotCount, markerIndexTree.Size(), "expected the trees map to have %d elements", slotCount)

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, 0)

	tf.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().EvictUntil(slotCount / 2)
	workers.WaitChildren()

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, slotCount/2)

	tf.Instance.(*inmemoryblockdag.BlockDAG).EvictionState().EvictUntil(slotCount)
	workers.WaitChildren()

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, slotCount)

	assert.Equal(t, 0, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to be empty")
	assert.Equal(t, 0, markerManager.markerBlockMappingEviction.Size(), "expected the marker block pruning map to be empty")
	assert.Equal(t, 0, markerManager.sequenceMarkersMapping.Size(), "expected the sequence marker tree map to be have 0 elements")
}

// We create slotCount blocks, each in a different slot and with a different marker, then we prune the markerManager and expect the mapping to be pruned accordingly.
func Test_BlockMarkerCeilingFloor(t *testing.T) {
	const blockCount = 100
	const markerGap = 100000

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	workers := workerpool.NewGroup(t.Name())
	tf := inmemoryblockdag.NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx slot.Index, prefix string) (block *blockdag.Block) {
		alias := fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			return blockdag.NewBlock(tf.CreateBlock(
				alias,
				models.WithIssuingTime(tf.SlotTimeProvider().GenesisTime()),
			))
		}
		return blockdag.NewBlock(tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(tf.SlotTimeProvider().StartTime(idx)),
		))
	}

	markerBlockMapping := make(map[markers.Marker]*blockdag.Block, blockCount)
	for i := slot.Index(1); i <= blockCount; i++ {
		blk := createNewBlock(i, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	for i := slot.Index(blockCount + markerGap); i <= 2*blockCount+markerGap; i++ {
		blk := createNewBlock(i-markerGap, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, 0)

	ceilingMarker, exists := markerManager.BlockCeiling(markers.NewMarker(1, blockCount))
	assert.True(t, exists, "ceiling marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount), ceilingMarker, "ceiling marker incorrect")

	ceilingMarker, exists = markerManager.BlockCeiling(markers.NewMarker(1, blockCount+1))
	assert.True(t, exists, "ceiling marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount+markerGap), ceilingMarker, "ceiling marker incorrect")

	ceilingMarker, exists = markerManager.BlockCeiling(markers.NewMarker(1, blockCount+markerGap-1))
	assert.True(t, exists, "ceiling marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount+markerGap), ceilingMarker, "ceiling marker incorrect")

	ceilingMarker, exists = markerManager.BlockCeiling(markers.NewMarker(1, 0))
	assert.True(t, exists, "ceiling marker should exist")
	assert.Equal(t, markers.NewMarker(1, 1), ceilingMarker, "ceiling marker incorrect")

	_, exists = markerManager.BlockCeiling(markers.NewMarker(1, 2*blockCount+markerGap+1))
	assert.False(t, exists, "ceiling marker should not exist")

	floorMarker, exists := markerManager.BlockFloor(markers.NewMarker(1, blockCount))
	assert.True(t, exists, "floor marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount), floorMarker, "floor marker incorrect")

	floorMarker, exists = markerManager.BlockFloor(markers.NewMarker(1, blockCount+1))
	assert.True(t, exists, "floor marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount), floorMarker, "floor marker incorrect")

	floorMarker, exists = markerManager.BlockFloor(markers.NewMarker(1, blockCount+markerGap-1))
	assert.True(t, exists, "floor marker should exist")
	assert.Equal(t, markers.NewMarker(1, blockCount), floorMarker, "floor marker incorrect")

	floorMarker, exists = markerManager.BlockFloor(markers.NewMarker(1, 2*blockCount+markerGap+100))
	assert.True(t, exists, "floor marker should exist")
	assert.Equal(t, markers.NewMarker(1, 2*blockCount+markerGap), floorMarker, "floor marker incorrect")

	_, exists = markerManager.BlockFloor(markers.NewMarker(1, 0))
	assert.False(t, exists, "floor marker should not exist")
}

// We create sequences for a slot X, each slot contains sequences <X; X+5>.
func Test_PruneSequences(t *testing.T) {
	const slotCount = 5
	const sequenceCount = 5
	const totalSequences = sequenceCount * slotCount
	const permanentSequenceID = markers.SequenceID(2)

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	// Create the sequence structure for the test. We creatte sequenceCount sequences for each of slotCount slots.
	// Each sequence X references the sequences X-2, X-1.
	// Each sequence is used only in a single slotIndex.
	{
		for sequenceSlot := 0; sequenceSlot < totalSequences; sequenceSlot++ {
			expectedSequenceID := markers.SequenceID(sequenceSlot)

			structureDetails := markers.NewStructureDetails()
			structureDetails.SetPastMarkerGap(100)

			switch expectedSequenceID {
			case 0:
				structureDetails.SetPastMarkers(markers.NewMarkers())
			case 1:
				structureDetails.SetPastMarkers(markers.NewMarkers(markers.NewMarker(0, 1)))
			default:
				structureDetails.SetPastMarkers(markers.NewMarkers(
					markers.NewMarker(expectedSequenceID-1, 1),
					markers.NewMarker(expectedSequenceID-2, 1),
				))
			}

			newStructureDetailsTmp, created := markerManager.SequenceManager.InheritStructureDetails([]*markers.StructureDetails{structureDetails}, false)

			// create another marker within the same sequence, so that in the next iteration a new sequence will be created
			newStructureDetails, _ := markerManager.SequenceManager.InheritStructureDetails([]*markers.StructureDetails{newStructureDetailsTmp}, false)

			assert.True(t, created, "expected to create a new sequence with sequence ID %d", expectedSequenceID)
			assert.True(t, newStructureDetails.IsPastMarker(), "expected the new sequence details to be past marker")
			assert.Equal(t, expectedSequenceID, newStructureDetails.PastMarkers().Marker().SequenceID())
			markerManager.registerSequenceEviction(slot.Index(expectedSequenceID/sequenceCount), expectedSequenceID)

			markerManager.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), utxo.NewTransactionIDs())
		}
	}

	// verify that the structure is correct
	{
		for sequenceID := markers.SequenceID(0); sequenceID < totalSequences; sequenceID++ {
			verifySequence(t, markerManager, sequenceID, -1, slotCount, sequenceCount, totalSequences)
		}
	}

	// simulate that slotIndex 2 is used in every slotIndex and in the future
	for slotIndex := 0; slotIndex < slotCount+1; slotIndex++ {
		markerManager.registerSequenceEviction(slot.Index(slotIndex), permanentSequenceID)
	}

	// verify that the structure is still correct
	for sequenceID := markers.SequenceID(0); sequenceID < totalSequences; sequenceID++ {
		verifySequence(t, markerManager, sequenceID, -1, slotCount, sequenceCount, totalSequences, permanentSequenceID)
	}

	// verify that the pruning is correct
	{
		for pruningSlot := 0; pruningSlot < 5; pruningSlot++ {
			markerManager.Evict(slot.Index(pruningSlot))

			startingSequence := markers.SequenceID((pruningSlot + 1) * sequenceCount)

			// check that the pruned sequences are gone
			_, sequencePruningExists := markerManager.sequenceEviction.Get(slot.Index(pruningSlot))
			assert.False(t, sequencePruningExists, "expected to not find a sequence pruning map for slotIndex %d", pruningSlot)

			for sequenceID := markers.SequenceID(0); sequenceID < startingSequence; sequenceID++ {
				if sequenceID == permanentSequenceID {
					continue
				}
				_, exists := markerManager.sequenceLastUsed.Get(sequenceID)
				assert.False(t, exists, "expected to not find a last used slotIndex for sequence %d", pruningSlot)

				_, exists = markerManager.SequenceManager.Sequence(sequenceID)
				assert.False(t, exists, "expected to not find sequence %d", sequenceID)

				_, exists = markerManager.markerIndexConflictIDMapping.Get(sequenceID)
				assert.False(t, exists, "expected to not find a conflict ID mapping for sequence %d", sequenceID)
			}

			// check that the remaining sequences are correct
			for sequenceID := startingSequence; sequenceID < totalSequences; sequenceID++ {
				verifySequence(t, markerManager, sequenceID, pruningSlot, slotCount, sequenceCount, totalSequences, permanentSequenceID)
			}
		}
	}

	// finally check that just permanentSequenceID is still there
	for i := markers.SequenceID(0); i < totalSequences; i++ {
		_, exists := markerManager.SequenceManager.Sequence(i)
		if i == permanentSequenceID {
			assert.True(t, exists, "expected to find sequence %d", i)
			lastUsedSlot, lastUsedExists := markerManager.sequenceLastUsed.Get(permanentSequenceID)
			assert.True(t, lastUsedExists, "expected to find a last used slotIndex for sequence %d", permanentSequenceID)
			assert.EqualValues(t, slotCount, lastUsedSlot, "expected the last used slotIndex to be %d but got %d", slotCount, lastUsedSlot)
		} else {
			assert.False(t, exists, "expected to not find sequence %d", i)
		}
	}
}

func verifySequence(t *testing.T, markerManager *MarkerManager[models.BlockID, *blockdag.Block], sequenceID markers.SequenceID, pruningSlot, slotCount, sequenceCount, totalSequences int, permanentSequenceID ...markers.SequenceID) {
	slotIndex := slot.Index(int(sequenceID) / sequenceCount)

	sequence, sequenceExists := markerManager.SequenceManager.Sequence(sequenceID)
	assert.True(t, sequenceExists, "expected to find sequence %d", sequenceID)

	validateReferencedMarkers(t, sequence, sequenceID, sequenceCount, pruningSlot)

	validateReferencingSequenceIDs(t, sequenceID, totalSequences, sequence.ReferencingSequences(), pruningSlot)

	_, mappingExists := markerManager.markerIndexConflictIDMapping.Get(sequenceID)
	assert.True(t, mappingExists, "expected to find a conflict ID mapping for sequence %d", sequenceID)

	lastUsedSlot, lastUsedExists := markerManager.sequenceLastUsed.Get(sequenceID)
	assert.True(t, lastUsedExists, "expected to find a last used slotIndex for sequence %d", sequenceID)
	expectedLastUsedSlot := slotIndex
	if len(permanentSequenceID) > 0 && sequenceID == permanentSequenceID[0] {
		expectedLastUsedSlot = slot.Index(slotCount)
	}
	assert.Equal(t, expectedLastUsedSlot, lastUsedSlot, "expected the last used slotIndex to be %d but got %d", slotIndex, lastUsedSlot)

	sequenceIDsUsed, sequencePruningMapExists := markerManager.sequenceEviction.Get(slotIndex)
	assert.True(t, sequencePruningMapExists, "expected to find a sequence pruning map for slotIndex %d", slotIndex)

	expectedSequenceSet := set.New[markers.SequenceID](false)
	if len(permanentSequenceID) > 0 {
		expectedSequenceSet.Add(permanentSequenceID[0])
	}
	for i := int(slotIndex) * sequenceCount; i < (int(slotIndex)*sequenceCount)+sequenceCount; i++ {
		expectedSequenceSet.Add(markers.SequenceID(i))
	}
	expectedSequenceSet.ForEach(func(sequenceIDExpected markers.SequenceID) {
		assert.True(t, sequenceIDsUsed.Has(sequenceIDExpected), "expected to find sequence %d in the sequence pruning map for slot %d, sequenceID %d", sequenceIDExpected, slotIndex, sequenceID)
	})
}

func validateReferencingSequenceIDs(t *testing.T, sequenceID markers.SequenceID, totalSequences int, actualSequences markers.SequenceIDs, maxPrunedSlot int) {
	expectedReferencingSequenceIDs := markers.NewSequenceIDs()
	if sequenceID < markers.SequenceID(totalSequences-2) {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(sequenceID+1, sequenceID+2)
	} else if sequenceID == markers.SequenceID(totalSequences-2) {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(sequenceID + 1)
	}
	assert.True(t, actualSequences.Equal(expectedReferencingSequenceIDs), "expected the referencing sequences to be %s but got %s", expectedReferencingSequenceIDs, actualSequences)
}

func validateReferencedMarkers(t *testing.T, sequence *markers.Sequence, sequenceID markers.SequenceID, sequenceCount, maxPrunedSlot int) {
	referencedMarkers := markers.NewMarkers()
	sequence.ReferencedMarkers(2).ForEach(func(referencedSequenceID markers.SequenceID, referencedIndex markers.Index) bool {
		referencedMarkers.Set(referencedSequenceID, referencedIndex)
		return true
	})

	expectedReferencedMarkers := markers.NewMarkers()
	if sequenceID-markers.SequenceID((maxPrunedSlot+1)*sequenceCount) >= 2 {
		expectedReferencedMarkers.Set(sequenceID-1, 1)
		expectedReferencedMarkers.Set(sequenceID-2, 1)
	} else if sequenceID-markers.SequenceID((maxPrunedSlot+1)*sequenceCount) == 1 {
		expectedReferencedMarkers.Set(sequenceID-1, 1)
	}
	assert.True(t, expectedReferencedMarkers.Equals(referencedMarkers), "expected the referenced markers for sequence %d to be %s but got %s", sequenceID, expectedReferencedMarkers, referencedMarkers)
}

func validateBlockMarkerMappingPruning(t *testing.T, markerBlockMapping map[markers.Marker]*blockdag.Block, markerManager *MarkerManager[models.BlockID, *blockdag.Block], prunedSlots int) {
	for marker, expectedBlock := range markerBlockMapping {
		mappedBlock, exists := markerManager.BlockFromMarker(marker)
		if expectedBlock.ID().SlotIndex <= slot.Index(prunedSlots) {
			assert.False(t, exists, "expected block %s with marker %s to be pruned", expectedBlock.ID(), marker)
			continue
		}
		assert.True(t, exists, "expected block %s with marker %s to be kept", expectedBlock.ID(), marker)
		assert.Equal(t, marker, lo.Return1(markerManager.BlockCeiling(marker)), "expected Ceiling to return the marker")
		assert.Equal(t, marker, lo.Return1(markerManager.BlockFloor(marker)), "expected Floor to return the marker")
		assert.Equal(t, expectedBlock.ID(), mappedBlock.ID(), "expected the marker %s to be mapped to block %s, but got %s", marker, expectedBlock.ID(), mappedBlock.ID())
	}
}
