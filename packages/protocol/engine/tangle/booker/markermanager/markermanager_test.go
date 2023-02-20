package markermanager

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker/markers"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/workerpool"
)

// We create epochCount blocks, each in a different epoch and with a different marker, then we prune the markerManager and expect the mapping to be pruned accordingly.
func Test_PruneMarkerBlockMapping(t *testing.T) {
	const epochCount = 100

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	workers := workerpool.NewGroup(t.Name())
	tf := blockdag.NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	event.Hook(tf.Instance.EvictionState.Events.EpochEvicted, markerManager.Evict)

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *blockdag.Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			return blockdag.NewBlock(tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			)), alias
		}
		return blockdag.NewBlock(tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx-1)*epoch.Duration, 0)),
		)), alias
	}

	markerBlockMapping := make(map[markers.Marker]*blockdag.Block, epochCount)
	for i := 1; i <= epochCount; i++ {
		blk, _ := createNewBlock(i, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	assert.Equal(t, epochCount, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to have %d elements", epochCount)
	assert.Equal(t, epochCount, markerManager.markerBlockMappingEviction.Size(), "expected the marker block pruning map to have %d elements", epochCount)
	assert.Equal(t, 1, markerManager.sequenceMarkersMapping.Size(), "expected the sequence marker tree map to be have 1 element")

	markerIndexTree, exists := markerManager.sequenceMarkersMapping.Get(1)
	assert.True(t, exists, "expected the sequence marker tree map to be contain tree map for SequenceID(1)")
	assert.Equal(t, epochCount, markerIndexTree.Size(), "expected the trees map to have %d elements", epochCount)

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, 0)

	tf.Instance.EvictionState.EvictUntil(epochCount / 2)
	workers.WaitAll()

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, epochCount/2)

	tf.Instance.EvictionState.EvictUntil(epochCount)
	workers.WaitAll()

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, epochCount)

	assert.Equal(t, 0, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to be empty")
	assert.Equal(t, 0, markerManager.markerBlockMappingEviction.Size(), "expected the marker block pruning map to be empty")
	assert.Equal(t, 0, markerManager.sequenceMarkersMapping.Size(), "expected the sequence marker tree map to be have 0 elements")
}

// We create epochCount blocks, each in a different epoch and with a different marker, then we prune the markerManager and expect the mapping to be pruned accordingly.
func Test_BlockMarkerCeilingFloor(t *testing.T) {
	const blockCount = 100
	const markerGap = 100000

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	workers := workerpool.NewGroup(t.Name())
	tf := blockdag.NewDefaultTestFramework(t, workers.CreateGroup("BlockDAGTestFramework"))

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *blockdag.Block) {
		alias := fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 1 {
			return blockdag.NewBlock(tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			))
		}
		return blockdag.NewBlock(tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx-1)*epoch.Duration, 0)),
		))
	}

	markerBlockMapping := make(map[markers.Marker]*blockdag.Block, blockCount)
	for i := 1; i <= blockCount; i++ {
		blk := createNewBlock(i, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	for i := blockCount + markerGap; i <= 2*blockCount+markerGap; i++ {
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

// We create sequences for an epoch X, each epoch contains sequences <X; X+5>.
func Test_PruneSequences(t *testing.T) {
	const epochCount = 5
	const sequenceCount = 5
	const totalSequences = sequenceCount * epochCount
	const permanentSequenceID = markers.SequenceID(2)

	markerManager := NewMarkerManager[models.BlockID, *blockdag.Block]()

	// Create the sequence structure for the test. We creatte sequenceCount sequences for each of epochCount epochs.
	// Each sequence X references the sequences X-2, X-1.
	// Each sequence is used only in a single epochIndex.
	{
		for sequenceEpoch := 0; sequenceEpoch < totalSequences; sequenceEpoch++ {
			expectedSequenceID := markers.SequenceID(sequenceEpoch)

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
			markerManager.registerSequenceEviction(epoch.Index(expectedSequenceID/sequenceCount), expectedSequenceID)

			markerManager.SetConflictIDs(newStructureDetails.PastMarkers().Marker(), utxo.NewTransactionIDs())
		}
	}

	// verify that the structure is correct
	{
		for sequenceID := markers.SequenceID(0); sequenceID < totalSequences; sequenceID++ {
			verifySequence(t, markerManager, sequenceID, -1, epochCount, sequenceCount, totalSequences)
		}
	}

	// simulate that epochIndex 2 is used in every epochIndex and in the future
	for epochIndex := 0; epochIndex < epochCount+1; epochIndex++ {
		markerManager.registerSequenceEviction(epoch.Index(epochIndex), permanentSequenceID)
	}

	// verify that the structure is still correct
	for sequenceID := markers.SequenceID(0); sequenceID < totalSequences; sequenceID++ {
		verifySequence(t, markerManager, sequenceID, -1, epochCount, sequenceCount, totalSequences, permanentSequenceID)
	}

	// verify that the pruning is correct
	{
		for pruningEpoch := 0; pruningEpoch < 5; pruningEpoch++ {
			markerManager.Evict(epoch.Index(pruningEpoch))

			startingSequence := markers.SequenceID((pruningEpoch + 1) * sequenceCount)

			// check that the pruned sequences are gone
			_, sequencePruningExists := markerManager.sequenceEviction.Get(epoch.Index(pruningEpoch))
			assert.False(t, sequencePruningExists, "expected to not find a sequence pruning map for epochIndex %d", pruningEpoch)

			for sequenceID := markers.SequenceID(0); sequenceID < startingSequence; sequenceID++ {
				if sequenceID == permanentSequenceID {
					continue
				}
				_, exists := markerManager.sequenceLastUsed.Get(sequenceID)
				assert.False(t, exists, "expected to not find a last used epochIndex for sequence %d", pruningEpoch)

				_, exists = markerManager.SequenceManager.Sequence(sequenceID)
				assert.False(t, exists, "expected to not find sequence %d", sequenceID)

				_, exists = markerManager.markerIndexConflictIDMapping.Get(sequenceID)
				assert.False(t, exists, "expected to not find a conflict ID mapping for sequence %d", sequenceID)
			}

			// check that the remaining sequences are correct
			for sequenceID := startingSequence; sequenceID < totalSequences; sequenceID++ {
				verifySequence(t, markerManager, sequenceID, pruningEpoch, epochCount, sequenceCount, totalSequences, permanentSequenceID)
			}
		}
	}

	// finally check that just permanentSequenceID is still there
	for i := markers.SequenceID(0); i < totalSequences; i++ {
		_, exists := markerManager.SequenceManager.Sequence(i)
		if i == permanentSequenceID {
			assert.True(t, exists, "expected to find sequence %d", i)
			lastUsedEpoch, lastUsedExists := markerManager.sequenceLastUsed.Get(permanentSequenceID)
			assert.True(t, lastUsedExists, "expected to find a last used epochIndex for sequence %d", permanentSequenceID)
			assert.EqualValues(t, epochCount, lastUsedEpoch, "expected the last used epochIndex to be %d but got %d", epochCount, lastUsedEpoch)
		} else {
			assert.False(t, exists, "expected to not find sequence %d", i)
		}
	}
}

func verifySequence(t *testing.T, markerManager *MarkerManager[models.BlockID, *blockdag.Block], sequenceID markers.SequenceID, pruningEpoch, epochCount, sequenceCount, totalSequences int, permanentSequenceID ...markers.SequenceID) {
	epochIndex := epoch.Index(int(sequenceID) / sequenceCount)

	sequence, sequenceExists := markerManager.SequenceManager.Sequence(sequenceID)
	assert.True(t, sequenceExists, "expected to find sequence %d", sequenceID)

	validateReferencedMarkers(t, sequence, sequenceID, sequenceCount, pruningEpoch)

	validateReferencingSequenceIDs(t, sequenceID, totalSequences, sequence.ReferencingSequences(), pruningEpoch)

	_, mappingExists := markerManager.markerIndexConflictIDMapping.Get(sequenceID)
	assert.True(t, mappingExists, "expected to find a conflict ID mapping for sequence %d", sequenceID)

	lastUsedEpoch, lastUsedExists := markerManager.sequenceLastUsed.Get(sequenceID)
	assert.True(t, lastUsedExists, "expected to find a last used epochIndex for sequence %d", sequenceID)
	expectedLastUsedEpoch := epochIndex
	if len(permanentSequenceID) > 0 && sequenceID == permanentSequenceID[0] {
		expectedLastUsedEpoch = epoch.Index(epochCount)
	}
	assert.Equal(t, expectedLastUsedEpoch, lastUsedEpoch, "expected the last used epochIndex to be %d but got %d", epochIndex, lastUsedEpoch)

	sequenceIDsUsed, sequencePruningMapExists := markerManager.sequenceEviction.Get(epochIndex)
	assert.True(t, sequencePruningMapExists, "expected to find a sequence pruning map for epochIndex %d", epochIndex)

	expectedSequenceSet := set.New[markers.SequenceID](false)
	if len(permanentSequenceID) > 0 {
		expectedSequenceSet.Add(permanentSequenceID[0])
	}
	for i := int(epochIndex) * sequenceCount; i < (int(epochIndex)*sequenceCount)+sequenceCount; i++ {
		expectedSequenceSet.Add(markers.SequenceID(i))
	}
	expectedSequenceSet.ForEach(func(sequenceIDExpected markers.SequenceID) {
		assert.True(t, sequenceIDsUsed.Has(sequenceIDExpected), "expected to find sequence %d in the sequence pruning map for epoch %d, sequenceID %d", sequenceIDExpected, epochIndex, sequenceID)
	})
}

func validateReferencingSequenceIDs(t *testing.T, sequenceID markers.SequenceID, totalSequences int, actualSequences markers.SequenceIDs, maxPrunedEpoch int) {
	expectedReferencingSequenceIDs := markers.NewSequenceIDs()
	if sequenceID < markers.SequenceID(totalSequences-2) {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(sequenceID+1, sequenceID+2)
	} else if sequenceID == markers.SequenceID(totalSequences-2) {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(sequenceID + 1)
	}
	assert.True(t, actualSequences.Equal(expectedReferencingSequenceIDs), "expected the referencing sequences to be %s but got %s", expectedReferencingSequenceIDs, actualSequences)
}

func validateReferencedMarkers(t *testing.T, sequence *markers.Sequence, sequenceID markers.SequenceID, sequenceCount, maxPrunedEpoch int) {
	referencedMarkers := markers.NewMarkers()
	sequence.ReferencedMarkers(2).ForEach(func(referencedSequenceID markers.SequenceID, referencedIndex markers.Index) bool {
		referencedMarkers.Set(referencedSequenceID, referencedIndex)
		return true
	})

	expectedReferencedMarkers := markers.NewMarkers()
	if sequenceID-markers.SequenceID((maxPrunedEpoch+1)*sequenceCount) >= 2 {
		expectedReferencedMarkers.Set(sequenceID-1, 1)
		expectedReferencedMarkers.Set(sequenceID-2, 1)
	} else if sequenceID-markers.SequenceID((maxPrunedEpoch+1)*sequenceCount) == 1 {
		expectedReferencedMarkers.Set(sequenceID-1, 1)
	}
	assert.True(t, expectedReferencedMarkers.Equals(referencedMarkers), "expected the referenced markers for sequence %d to be %s but got %s", sequenceID, expectedReferencedMarkers, referencedMarkers)
}

func validateBlockMarkerMappingPruning(t *testing.T, markerBlockMapping map[markers.Marker]*blockdag.Block, markerManager *MarkerManager[models.BlockID, *blockdag.Block], prunedEpochs int) {
	for marker, expectedBlock := range markerBlockMapping {
		mappedBlock, exists := markerManager.BlockFromMarker(marker)
		if expectedBlock.ID().EpochIndex <= epoch.Index(prunedEpochs) {
			assert.False(t, exists, "expected block %s with marker %s to be pruned", expectedBlock.ID(), marker)
			continue
		}
		assert.True(t, exists, "expected block %s with marker %s to be kept", expectedBlock.ID(), marker)
		assert.Equal(t, marker, lo.Return1(markerManager.BlockCeiling(marker)), "expected Ceiling to return the marker")
		assert.Equal(t, marker, lo.Return1(markerManager.BlockFloor(marker)), "expected Floor to return the marker")
		assert.Equal(t, expectedBlock.ID(), mappedBlock.ID(), "expected the marker %s to be mapped to block %s, but got %s", marker, expectedBlock.ID(), mappedBlock.ID())
	}
}
