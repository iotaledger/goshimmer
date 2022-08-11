package booker

import (
	"fmt"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/core/markers"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/core/tangle/models"
)

// We create epochCount blocks, each in a different epoch and with a different marker, then we prune the markerManager and expect the mapping to be pruned accordingly.
func Test_PruneMarkerBlockMapping(t *testing.T) {
	const epochCount = 100

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	markerManager := tf.Booker.markerManager

	// create a helper function that creates the blocks
	createNewBlock := func(idx int, prefix string) (block *Block, alias string) {
		alias = fmt.Sprintf("blk%s-%d", prefix, idx)
		if idx == 0 {
			return NewBlock(tangle.NewBlock(tf.CreateBlock(
				alias,
				models.WithIssuingTime(time.Unix(epoch.GenesisTime, 0)),
			))), alias
		}
		return NewBlock(tangle.NewBlock(tf.CreateBlock(
			alias,
			models.WithStrongParents(tf.BlockIDs(fmt.Sprintf("blk%s-%d", prefix, idx-1))),
			models.WithIssuingTime(time.Unix(epoch.GenesisTime+int64(idx)*epoch.Duration, 0)),
		))), alias
	}

	markerBlockMapping := make(map[markers.Marker]*Block, epochCount)
	for i := 0; i < epochCount; i++ {
		blk, _ := createNewBlock(i, "")
		markerBlockMapping[markers.NewMarker(1, markers.Index(i))] = blk
		markerManager.addMarkerBlockMapping(markers.NewMarker(1, markers.Index(i)), blk)
	}

	assert.Equal(t, epochCount, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to be empty")
	assert.Equal(t, epochCount, markerManager.markerBlockMappingPruning.Size(), "expected the marker block pruning map to be empty")

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, 0)

	markerManager.Prune(epochCount / 2)

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, epochCount/2)

	markerManager.Prune(epochCount)

	validateBlockMarkerMappingPruning(t, markerBlockMapping, markerManager, epochCount)

	assert.Equal(t, 0, markerManager.markerBlockMapping.Size(), "expected the marker block mapping to be empty")
	assert.Equal(t, 0, markerManager.markerBlockMappingPruning.Size(), "expected the marker block pruning map to be empty")
}

// We create sequences for an epoch X, each epoch contains sequences <X; X+5>
func Test_PruneSequences(t *testing.T) {
	const epochCount = 5
	const sequenceCount = 5
	const totalSequences = sequenceCount * epochCount

	epoch.GenesisTime = time.Now().Unix() - epochCount*epoch.Duration

	tf := NewTestFramework(t)
	defer tf.Shutdown()

	markerManager := tf.Booker.markerManager

	// Create the sequence structure for the test. We creatte sequenceCount sequences for each of epochCount epochs.
	// Each sequence X references the sequences X-2, X-1.
	// Each sequence is used only in a single epoch.
	// TODO: add a sequence that is used in multiple epochs.
	{
		for sequenceEpoch := 0; sequenceEpoch < totalSequences; sequenceEpoch++ {
			expectedSequenceID := markers.SequenceID(sequenceEpoch)

			structureDetails := markers.NewStructureDetails()
			structureDetails.SetPastMarkerGap(100)

			if expectedSequenceID >= 2 {
				structureDetails.SetPastMarkers(markers.NewMarkers(
					markers.NewMarker(expectedSequenceID-1, 1),
					markers.NewMarker(expectedSequenceID-2, 1),
				))
			} else if expectedSequenceID == 1 {
				structureDetails.SetPastMarkers(markers.NewMarkers(
					markers.NewMarker(markers.SequenceID(0), 1),
				))
			} else {
				continue
			}

			newStructureDetails, created := markerManager.sequenceManager.InheritStructureDetails([]*markers.StructureDetails{structureDetails})

			assert.True(t, created, "expected to create a new sequence details")
			assert.True(t, newStructureDetails.IsPastMarker(), "expected the new sequence details to be past marker")
			assert.Equal(t, expectedSequenceID, newStructureDetails.PastMarkers().Marker().SequenceID())
			markerManager.registerSequencePruning(epoch.Index(expectedSequenceID/sequenceCount), expectedSequenceID)

			markerManager.setConflictIDs(newStructureDetails.PastMarkers().Marker(), utxo.NewTransactionIDs())
		}
	}

	// verify that the structure is correct
	{
		for sequenceID := 0; sequenceID < totalSequences; sequenceID++ {
			sequence, sequenceExists := markerManager.sequenceManager.Sequence(markers.SequenceID(sequenceID))
			assert.True(t, sequenceExists, "expected to find sequence %d", sequenceID)

			validateReferencedMarkers(t, sequence, sequenceID, sequenceCount, -1)

			validateReferencingSequenceIDs(t, sequenceID, totalSequences, sequence.ReferencingSequences(), -1)

			_, mappingExists := markerManager.markerIndexConflictIDMapping.Get(markers.SequenceID(sequenceID))
			assert.True(t, mappingExists, "expected to find a conflict ID mapping for sequence %d", sequenceID)

			lastUsedEpoch, lastUsedExists := markerManager.sequenceLastUsed.Get(markers.SequenceID(sequenceID))
			assert.True(t, lastUsedExists, "expected to find a last used epoch for sequence %d", sequenceID)
			assert.EqualValues(t, sequenceID/sequenceCount, lastUsedEpoch, "expected the last used epoch to be %d but got %d", sequenceID/sequenceCount, lastUsedEpoch)

			sequenceIDsUsed, sequencePruningMapExists := markerManager.sequencePruning.Get(epoch.Index(sequenceID / sequenceCount))
			assert.True(t, sequencePruningMapExists, "expected to find a sequence pruning map for epoch %d", sequenceID/sequenceCount)
			assert.EqualValues(t, sequenceID/sequenceCount, lastUsedEpoch, "expected the last used epoch to be %d but got %d", sequenceID/sequenceCount, lastUsedEpoch)
			assert.Equal(t, 5, sequenceIDsUsed.Size(), "expected the sequence pruning map to have size 5")

			sequenceIDsUsed.ForEach(func(sequenceIDused markers.SequenceID) {
				assert.EqualValues(t, sequenceID/sequenceCount, sequenceIDused/sequenceCount, "expected the sequence ID used to be %d but got %d", sequenceID, sequenceIDused)
			})

		}
	}

	for epochIndex := 0; epochIndex < epochCount; epochIndex++ {
		markerManager.registerSequencePruning(epoch.Index(epochIndex), markers.SequenceID(2))
	}

	// verify that the pruning is correct
	{
		for pruningEpoch := 0; pruningEpoch < 1; pruningEpoch++ {
			markerManager.Prune(epoch.Index(pruningEpoch))

			// check that the remaining sequences are correct
			startingSequence := (pruningEpoch + 1) * sequenceCount

			for sequenceID := startingSequence; sequenceID < totalSequences; sequenceID++ {
				epochIndex := sequenceID / sequenceCount

				sequence, sequenceExists := markerManager.sequenceManager.Sequence(markers.SequenceID(sequenceID))
				assert.True(t, sequenceExists, "expected to find sequence %d", sequenceID)

				validateReferencedMarkers(t, sequence, sequenceID, sequenceCount, pruningEpoch)

				validateReferencingSequenceIDs(t, sequenceID, totalSequences, sequence.ReferencingSequences(), pruningEpoch)

				_, mappingExists := markerManager.markerIndexConflictIDMapping.Get(markers.SequenceID(sequenceID))
				assert.True(t, mappingExists, "expected to find a conflict ID mapping for sequence %d", sequenceID)

				lastUsedEpoch, lastUsedExists := markerManager.sequenceLastUsed.Get(markers.SequenceID(sequenceID))
				assert.True(t, lastUsedExists, "expected to find a last used epoch for sequence %d", sequenceID)
				assert.EqualValues(t, epochIndex, lastUsedEpoch, "expected the last used epoch to be %d but got %d", epochIndex, lastUsedEpoch)

				sequenceIDsUsed, sequencePruningMapExists := markerManager.sequencePruning.Get(epoch.Index(epochIndex))
				assert.True(t, sequencePruningMapExists, "expected to find a sequence pruning map for epoch %d", epochIndex)
				assert.EqualValues(t, epochIndex, lastUsedEpoch, "expected the last used epoch to be %d but got %d", epochIndex, lastUsedEpoch)

				expectedSequenceSet := set.New[markers.SequenceID](false)
				expectedSequenceSet.Add(markers.SequenceID(2))
				for i := sequenceID / sequenceCount; i < (sequenceID/sequenceCount)+sequenceCount; i++ {
					expectedSequenceSet.Add(markers.SequenceID(i + epochIndex))
				}
				expectedSequenceSet.ForEach(func(sequenceIDExpected markers.SequenceID) {
					assert.True(t, sequenceIDsUsed.Has(sequenceIDExpected), "expected to find sequence %d in the sequence pruning map", sequenceIDExpected)
				})

			}

			// check that the pruned sequences are gone
			_, sequencePruningExists := markerManager.sequencePruning.Get(epoch.Index(pruningEpoch))
			assert.False(t, sequencePruningExists, "expected to not find a sequence pruning map for epoch %d", pruningEpoch)

			for sequenceID := 0; sequenceID < startingSequence; sequenceID++ {
				_, exists := markerManager.sequenceLastUsed.Get(markers.SequenceID(sequenceID))
				assert.False(t, exists, "expected to not find a last used epoch for sequence %d", pruningEpoch)

				_, exists = markerManager.sequenceManager.Sequence(markers.SequenceID(sequenceID))
				assert.False(t, exists, "expected to not find sequence %d", sequenceID)

				_, exists = markerManager.markerIndexConflictIDMapping.Get(markers.SequenceID(sequenceID))
				assert.False(t, exists, "expected to not find a conflict ID mapping for sequence %d", sequenceID)

			}
		}
	}
}

func validateReferencingSequenceIDs(t *testing.T, sequenceID int, totalSequences int, actualSequences markers.SequenceIDs, maxPrunedEpoch int) {
	expectedReferencingSequenceIDs := markers.NewSequenceIDs()
	if sequenceID < totalSequences-2 {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(markers.SequenceID(sequenceID+1), markers.SequenceID(sequenceID+2))
	} else if sequenceID == totalSequences-2 {
		expectedReferencingSequenceIDs = markers.NewSequenceIDs(markers.SequenceID(sequenceID + 1))
	}
	assert.True(t, actualSequences.Equal(expectedReferencingSequenceIDs), "expected the referencing sequences to be %s but got %s", expectedReferencingSequenceIDs, actualSequences)
}

func validateReferencedMarkers(t *testing.T, sequence *markers.Sequence, sequenceID, sequenceCount, maxPrunedEpoch int) {
	referencedMarkers := markers.NewMarkers()
	sequence.ReferencedMarkers(2).ForEach(func(referencedSequenceID markers.SequenceID, referencedIndex markers.Index) bool {
		referencedMarkers.Set(referencedSequenceID, referencedIndex)
		return true
	})

	expectedReferencedMarkers := markers.NewMarkers()
	if sequenceID-(maxPrunedEpoch+1)*sequenceCount >= 2 {
		expectedReferencedMarkers.Set(markers.SequenceID(sequenceID-1), 1)
		expectedReferencedMarkers.Set(markers.SequenceID(sequenceID-2), 1)
	} else if sequenceID-(maxPrunedEpoch+1)*sequenceCount == 1 {
		expectedReferencedMarkers.Set(markers.SequenceID(sequenceID-1), 1)
	}
	fmt.Println(sequenceID, maxPrunedEpoch, expectedReferencedMarkers, referencedMarkers)
	assert.True(t, expectedReferencedMarkers.Equals(referencedMarkers), "expected the referenced markers for sequence %d to be %s but got %s", sequenceID, expectedReferencedMarkers, referencedMarkers)
}

func validateBlockMarkerMappingPruning(t *testing.T, markerBlockMapping map[markers.Marker]*Block, markerManager *MarkerManager, prunedEpochs int) {
	for marker, expectedBlock := range markerBlockMapping {
		mappedBlock, exists := markerManager.BlockFromMarker(marker)
		if expectedBlock.ID().EpochIndex <= epoch.Index(prunedEpochs) {
			assert.False(t, exists, "expected block %s with marker %s to be pruned", expectedBlock.ID(), marker)
			continue
		}
		assert.True(t, exists, "expected block %s with marker %s to be kept", expectedBlock.ID(), marker)
		assert.Equal(t, expectedBlock.ID(), mappedBlock.ID(), "expected the marker %s to be mapped to block %s, but got %s", marker, expectedBlock.ID(), mappedBlock.ID())
	}
}
