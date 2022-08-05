package marker

import (
	"fmt"
	"math"
	"sync"

	"github.com/iotaledger/hive.go/core/generics/options"

	"github.com/iotaledger/hive.go/core/generics/walker"

	"github.com/iotaledger/goshimmer/packages/core/memstorage"
)

// region SequenceManager //////////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceManager is the managing entity for the Marker related business logic. It is stateful and automatically stores its
// state in a memstorage.
type SequenceManager struct {
	sequences              *memstorage.Storage[SequenceID, *Sequence]
	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex

	// MaxPastMarkerDistance is a parameter for the SequenceManager that allows to specify how many consecutive blocks are
	// allowed to not receive a new PastMaster before we create a new Sequence.
	maxPastMarkerDistance uint64
}

// NewSequenceManager is the constructor of the SequenceManager that takes a KVStore to persist its state.
func NewSequenceManager(opts ...options.Option[SequenceManager]) (m *SequenceManager) {
	m = &SequenceManager{
		maxPastMarkerDistance: 30,
		sequences:             memstorage.New[SequenceID, *Sequence](),
	}
	options.Apply(m, opts)

	m.sequences.Set(0, NewSequence(0, NewMarkers()))

	return m
}

// InheritStructureDetails takes the StructureDetails of the referenced parents and returns new StructureDetails for the
// block that was just added to the DAG. It automatically creates a new Sequence and Index if necessary and returns an
// additional flag that indicates if a new Sequence was created.
// InheritStructureDetails inherits the structure details of the given parent StructureDetails.
func (m *SequenceManager) InheritStructureDetails(referencedStructureDetails []*StructureDetails) (inheritedStructureDetails *StructureDetails, newSequenceCreated bool) {
	inheritedStructureDetails = m.mergeParentStructureDetails(referencedStructureDetails)

	inheritedStructureDetails.SetPastMarkers(m.normalizeMarkers(inheritedStructureDetails.PastMarkers()))
	if inheritedStructureDetails.PastMarkers().Size() == 0 {
		inheritedStructureDetails.SetPastMarkers(NewMarkers(NewMarker(0, 0)))
	}

	assignedMarker, sequenceExtended := m.extendHighestAvailableSequence(inheritedStructureDetails.PastMarkers())
	if !sequenceExtended {
		newSequenceCreated, assignedMarker = m.createSequenceIfNecessary(inheritedStructureDetails)
	}

	if !sequenceExtended && !newSequenceCreated {
		return inheritedStructureDetails, false
	}

	inheritedStructureDetails.SetIsPastMarker(true)
	inheritedStructureDetails.SetPastMarkerGap(0)
	inheritedStructureDetails.SetPastMarkers(NewMarkers(assignedMarker))

	return inheritedStructureDetails, newSequenceCreated
}

// Sequence retrieves a Sequence by its ID.
func (m *SequenceManager) Sequence(sequenceID SequenceID) (sequence *Sequence, exists bool) {
	return m.sequences.Get(sequenceID)
}

// mergeParentStructureDetails merges the information of a set of parent StructureDetails into a single StructureDetails
// object.
func (m *SequenceManager) mergeParentStructureDetails(referencedStructureDetails []*StructureDetails) (mergedStructureDetails *StructureDetails) {
	mergedStructureDetails = NewStructureDetails()
	mergedStructureDetails.SetPastMarkerGap(math.MaxUint64)

	for _, referencedMarkerPair := range referencedStructureDetails {
		mergedStructureDetails.PastMarkers().Merge(referencedMarkerPair.PastMarkers())

		if referencedMarkerPair.PastMarkerGap() < mergedStructureDetails.PastMarkerGap() {
			mergedStructureDetails.SetPastMarkerGap(referencedMarkerPair.PastMarkerGap())
		}

		if referencedMarkerPair.Rank() > mergedStructureDetails.Rank() {
			mergedStructureDetails.SetRank(referencedMarkerPair.Rank())
		}
	}

	mergedStructureDetails.SetPastMarkerGap(mergedStructureDetails.PastMarkerGap() + 1)
	mergedStructureDetails.SetRank(mergedStructureDetails.Rank() + 1)

	return mergedStructureDetails
}

// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *SequenceManager) normalizeMarkers(markers *Markers) (normalizedMarkers *Markers) {
	normalizedMarkers = markers.Clone()

	normalizeWalker := walker.New[Marker]()
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		normalizeWalker.Push(NewMarker(sequenceID, index))

		return true
	})

	seenMarkers := NewMarkers()
	for i := 0; normalizeWalker.HasNext(); i++ {
		currentMarker := normalizeWalker.Next()

		if i >= markers.Size() {
			if added, updated := seenMarkers.Set(currentMarker.SequenceID(), currentMarker.Index()); !added && !updated {
				continue
			}

			index, exists := normalizedMarkers.Get(currentMarker.SequenceID())
			if exists {
				if index > currentMarker.Index() {
					continue
				}

				normalizedMarkers.Delete(currentMarker.SequenceID())
			}
		}

		sequence, exists := m.Sequence(currentMarker.SequenceID())
		if !exists {
			panic(fmt.Sprintf("failed to load Sequence with %s", currentMarker.SequenceID()))
		}

		sequence.ReferencedMarkers(currentMarker.Index()).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
			normalizeWalker.Push(NewMarker(referencedSequenceID, referencedIndex))
			return true
		})
	}

	return normalizedMarkers
}

// extendHighestAvailableSequence is an internal utility function that tries to extend the referenced Sequences in
// descending order. It returns the newly assigned Marker and a boolean value that indicates if one of the referenced
// Sequences could be extended.
func (m *SequenceManager) extendHighestAvailableSequence(referencedPastMarkers *Markers) (marker Marker, extended bool) {
	referencedPastMarkers.ForEachSorted(func(sequenceID SequenceID, index Index) bool {
		sequence, exists := m.Sequence(sequenceID)
		if !exists {
			panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
		}

		if newIndex, remainingReferencedPastMarkers, sequenceExtended := sequence.TryExtend(referencedPastMarkers); sequenceExtended {
			extended = sequenceExtended
			marker = NewMarker(sequenceID, newIndex)

			m.registerReferencingMarker(remainingReferencedPastMarkers, marker)
		}

		return !extended
	})

	return
}

// createSequenceIfNecessary is an internal utility function that creates a new Sequence if the distance to the last
// past Marker is higher or equal than the configured threshold and returns the first Marker in that Sequence.
func (m *SequenceManager) createSequenceIfNecessary(structureDetails *StructureDetails) (created bool, firstMarker Marker) {
	if structureDetails.PastMarkerGap() < m.maxPastMarkerDistance {
		return
	}

	m.sequenceIDCounterMutex.Lock()
	m.sequenceIDCounter++
	newSequence := NewSequence(m.sequenceIDCounter, structureDetails.PastMarkers())
	m.sequenceIDCounterMutex.Unlock()

	m.sequences.Set(newSequence.ID(), newSequence)

	firstMarker = NewMarker(newSequence.ID(), newSequence.LowestIndex())

	m.registerReferencingMarker(structureDetails.PastMarkers(), firstMarker)

	return true, firstMarker
}

// laterMarkersReferenceEarlierMarkers is an internal utility function that returns true if the later Markers reference
// the earlier Markers. If requireBiggerMarkers is false then a Marker with an equal Index is considered to be a valid
// reference.
func (m *SequenceManager) laterMarkersReferenceEarlierMarkers(laterMarkers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
	referenceWalker := walker.New[Marker]()
	laterMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		referenceWalker.Push(NewMarker(sequenceID, index))
		return true
	})

	seenMarkers := NewMarkers()
	for i := 0; referenceWalker.HasNext(); i++ {
		laterMarker := referenceWalker.Next()
		if added, updated := seenMarkers.Set(laterMarker.SequenceID(), laterMarker.Index()); !added && !updated {
			continue
		}

		isInitialLaterMarker := i < laterMarkers.Size()
		if m.laterMarkerDirectlyReferencesEarlierMarkers(laterMarker, earlierMarkers, isInitialLaterMarker && requireBiggerMarkers) {
			return true
		}

		sequence, exists := m.Sequence(laterMarker.SequenceID())
		if !exists {
			panic(fmt.Sprintf("failed to load Sequence with %s", laterMarker.SequenceID()))
		}

		sequence.ReferencedMarkers(laterMarker.Index()).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
			referenceWalker.Push(NewMarker(referencedSequenceID, referencedIndex))
			return true
		})
	}

	return false
}

// laterMarkerDirectlyReferencesEarlierMarkers returns true if the later Marker directly references the earlier Markers.
func (m *SequenceManager) laterMarkerDirectlyReferencesEarlierMarkers(laterMarker Marker, earlierMarkers *Markers, requireBiggerMarkers bool) bool {
	earlierMarkersLowestIndex := earlierMarkers.LowestIndex()
	if requireBiggerMarkers {
		earlierMarkersLowestIndex++
	}

	if laterMarker.Index() < earlierMarkersLowestIndex {
		return false
	}

	earlierIndex, sequenceExists := earlierMarkers.Get(laterMarker.SequenceID())
	if !sequenceExists {
		return false
	}

	if requireBiggerMarkers {
		earlierIndex++
	}

	return laterMarker.Index() >= earlierIndex
}

// registerReferencingMarker is an internal utility function that adds a referencing Marker to the internal data
// structure.
func (m *SequenceManager) registerReferencingMarker(referencedPastMarkers *Markers, marker Marker) {
	referencedPastMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		sequence, exists := m.Sequence(sequenceID)
		if !exists {
			panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
		}

		sequence.AddReferencingMarker(index, marker)
		return true
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Options //////////////////////////////////////////////////////////////////////////////////////////////////////

// WithMaxPastMarkerDistance is an Option for the SequenceManager that allows to specify how many consecutive blocks are
// allowed to not receive a new PastMaster before we create a new Sequence.
func WithMaxPastMarkerDistance(distance uint64) options.Option[SequenceManager] {
	return func(options *SequenceManager) {
		options.maxPastMarkerDistance = distance
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
