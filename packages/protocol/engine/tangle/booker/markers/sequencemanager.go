package markers

import (
	"fmt"
	"math"
	"sync"

	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/runtime/options"
)

// region SequenceManager //////////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceManager is the managing entity for the Marker related business logic. It is stateful and automatically stores its
// state in a memstorage.
type SequenceManager struct {
	sequences              *shrinkingmap.ShrinkingMap[SequenceID, *Sequence]
	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex

	// optsMaxPastMarkerDistance is a parameter for the SequenceManager that allows to specify how many consecutive blocks are
	// allowed to not receive a new PastMaster before we create a new Sequence.
	optsMaxPastMarkerDistance uint64
	optsIncreaseIndexCallback IncreaseIndexCallback
}

// NewSequenceManager is the constructor of the SequenceManager that takes a KVStore to persist its state.
func NewSequenceManager(opts ...options.Option[SequenceManager]) (m *SequenceManager) {
	m = options.Apply(&SequenceManager{
		optsMaxPastMarkerDistance: 30,
		sequences:                 shrinkingmap.New[SequenceID, *Sequence](),
		optsIncreaseIndexCallback: func(SequenceID, Index) bool {
			return true
		},
	}, opts)

	return m
}

// InheritStructureDetails takes the StructureDetails of the referenced parents and returns new StructureDetails for the
// block that was just added to the DAG. It automatically creates a new Sequence and Index if necessary and returns an
// additional flag that indicates if a new Sequence was created. When attaching to one of the root blocks, a new
// sequence is always created and the block is assigned Index(1), while Index(0) is a root index.
// InheritStructureDetails inherits the structure details of the given parent StructureDetails.
func (s *SequenceManager) InheritStructureDetails(referencedStructureDetails []*StructureDetails, allParentsInPastSlot bool) (inheritedStructureDetails *StructureDetails, newSequenceCreated bool) {
	inheritedStructureDetails = s.mergeParentStructureDetails(referencedStructureDetails)

	inheritedStructureDetails.SetPastMarkers(inheritedStructureDetails.PastMarkers())

	if inheritedStructureDetails.PastMarkers().Size() == 0 {
		// call createSequence without any past markers just so the sequence is created for later use.
		// pastMarkers of inheritedStructureDetails will be overridden with the actual marker later.
		inheritedStructureDetails.SetPastMarkers(NewMarkers(s.createSequence(NewMarkers())))
		newSequenceCreated = true
	}

	assignedMarker, sequenceExtended := s.extendHighestAvailableSequence(inheritedStructureDetails.PastMarkers())
	if !sequenceExtended && !newSequenceCreated {
		newSequenceCreated, assignedMarker = s.createSequenceIfNecessary(inheritedStructureDetails, allParentsInPastSlot)
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
func (s *SequenceManager) Sequence(sequenceID SequenceID) (sequence *Sequence, exists bool) {
	return s.sequences.Get(sequenceID)
}

func (s *SequenceManager) Delete(id SequenceID) {
	sequence, sequenceExists := s.Sequence(id)
	if !sequenceExists {
		return
	}

	// iterate through all the sequences referencing this sequence and remove the reference
	for it := sequence.ReferencingSequences().Iterator(); it.HasNext(); {
		if referencingSequence, referencingSequenceExists := s.Sequence(it.Next()); referencingSequenceExists {
			referencingSequence.referencedMarkers.Delete(id)
		}
	}
	s.sequences.Delete(id)
}

func (s *SequenceManager) SetIncreaseIndexCallback(callback IncreaseIndexCallback) {
	s.optsIncreaseIndexCallback = callback
}

// mergeParentStructureDetails merges the information of a set of parent StructureDetails into a single StructureDetails
// object.
func (s *SequenceManager) mergeParentStructureDetails(referencedStructureDetails []*StructureDetails) (mergedStructureDetails *StructureDetails) {
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
func (s *SequenceManager) normalizeMarkers(markers *Markers) (normalizedMarkers *Markers) {
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

		sequence, exists := s.Sequence(currentMarker.SequenceID())
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
func (s *SequenceManager) extendHighestAvailableSequence(referencedPastMarkers *Markers) (marker Marker, extended bool) {
	referencedPastMarkers.ForEachSorted(func(sequenceID SequenceID, index Index) bool {
		sequence, exists := s.Sequence(sequenceID)
		if !exists {
			panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
		}

		if newIndex, remainingReferencedPastMarkers, sequenceExtended := sequence.TryExtend(referencedPastMarkers, s.optsIncreaseIndexCallback); sequenceExtended {
			extended = sequenceExtended
			marker = NewMarker(sequenceID, newIndex)
			s.registerReferencingMarker(remainingReferencedPastMarkers, marker)
		}

		return !extended
	})

	return
}

// createSequenceIfNecessary is an internal utility function that creates a new Sequence if the distance to the last
// past Marker is higher or equal than the configured threshold and returns the first Marker in that Sequence.
func (s *SequenceManager) createSequenceIfNecessary(structureDetails *StructureDetails, allParentsInPastSlot bool) (created bool, firstMarker Marker) {
	if !allParentsInPastSlot && structureDetails.PastMarkerGap() < s.optsMaxPastMarkerDistance {
		return
	}
	return true, s.createSequence(structureDetails.PastMarkers())
}

// createSequenceIfNecessary is an internal utility function that creates a new Sequence.
func (s *SequenceManager) createSequence(referencedMarkers *Markers) (firstMarker Marker) {
	s.sequenceIDCounterMutex.Lock()
	newSequence := NewSequence(s.sequenceIDCounter, referencedMarkers)
	s.sequenceIDCounter++
	s.sequenceIDCounterMutex.Unlock()

	s.sequences.Set(newSequence.ID(), newSequence)
	firstMarker = NewMarker(newSequence.ID(), newSequence.LowestIndex())

	s.registerReferencingMarker(referencedMarkers, firstMarker)

	return firstMarker
}

// laterMarkersReferenceEarlierMarkers is an internal utility function that returns true if the later Markers reference
// the earlier Markers. If requireBiggerMarkers is false then a Marker with an equal Index is considered to be a valid
// reference.
func (s *SequenceManager) laterMarkersReferenceEarlierMarkers(laterMarkers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
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
		if s.laterMarkerDirectlyReferencesEarlierMarkers(laterMarker, earlierMarkers, isInitialLaterMarker && requireBiggerMarkers) {
			return true
		}

		sequence, exists := s.Sequence(laterMarker.SequenceID())
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
func (s *SequenceManager) laterMarkerDirectlyReferencesEarlierMarkers(laterMarker Marker, earlierMarkers *Markers, requireBiggerMarkers bool) bool {
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
func (s *SequenceManager) registerReferencingMarker(referencedPastMarkers *Markers, marker Marker) {
	referencedPastMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		sequence, exists := s.Sequence(sequenceID)
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
		options.optsMaxPastMarkerDistance = distance
	}
}

// WithIncreaseIndexCallback is an Option for the SequenceManager that allows to specify whether a new marker index
// should be created. This is especially useful for testing.
func WithIncreaseIndexCallback(callback IncreaseIndexCallback) options.Option[SequenceManager] {
	return func(options *SequenceManager) {
		options.optsIncreaseIndexCallback = callback
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
