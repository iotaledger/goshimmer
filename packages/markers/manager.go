package markers

import (
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the managing entity for the Marker related business logic. It is stateful and automatically stores its
// state in an underlying KVStore.
type Manager struct {
	store                     kvstore.KVStore
	sequenceStore             *objectstorage.ObjectStorage
	sequenceAliasMappingStore *objectstorage.ObjectStorage
	sequenceIDCounter         SequenceID
	sequenceIDCounterMutex    sync.Mutex
	shutdownOnce              sync.Once
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(store kvstore.KVStore) (newManager *Manager) {
	sequenceIDCounter := SequenceID(1)
	if storedSequenceIDCounter, err := store.Get(kvstore.Key("sequenceIDCounter")); err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	} else if storedSequenceIDCounter != nil {
		if sequenceIDCounter, _, err = SequenceIDFromBytes(storedSequenceIDCounter); err != nil {
			panic(err)
		}
	}

	osFactory := objectstorage.NewFactory(store, database.PrefixMarkers)
	newManager = &Manager{
		store:                     store,
		sequenceStore:             osFactory.New(PrefixSequence, SequenceFromObjectStorage, objectStorageOptions...),
		sequenceAliasMappingStore: osFactory.New(PrefixSequenceAliasMapping, SequenceAliasMappingFromObjectStorage, objectStorageOptions...),
		sequenceIDCounter:         sequenceIDCounter,
	}

	if cachedSequence, stored := newManager.sequenceStore.StoreIfAbsent(NewSequence(SequenceID(0), NewMarkers(), 0)); stored {
		cachedSequence.Release()
	}

	return
}

// InheritStructureDetails takes the StructureDetails of the referenced parents and returns new StructureDetails for the
// message that was just added to the DAG. It automatically creates a new Sequence and Index if necessary and returns an
// additional flag that indicates if a new Sequence was created.
func (m *Manager) InheritStructureDetails(referencedStructureDetails []*StructureDetails, increaseIndexCallback IncreaseIndexCallback, sequenceAlias SequenceAlias) (inheritedStructureDetails *StructureDetails, newSequenceCreated bool) {
	inheritedStructureDetails = &StructureDetails{
		FutureMarkers: NewMarkers(),
	}

	// merge parent's pastMarkers
	mergedPastMarkers := NewMarkers()
	for _, referencedMarkerPair := range referencedStructureDetails {
		mergedPastMarkers.Merge(referencedMarkerPair.PastMarkers)
		// update highest rank
		if referencedMarkerPair.Rank > inheritedStructureDetails.Rank {
			inheritedStructureDetails.Rank = referencedMarkerPair.Rank
		}
	}
	// rank for this message is set to highest rank of parents + 1
	inheritedStructureDetails.Rank++

	normalizedMarkers, referencedSequences := m.normalizeMarkers(mergedPastMarkers)
	rankOfReferencedSequences := normalizedMarkers.HighestRank()
	referencedMarkers, referencedMarkersExist := normalizedMarkers.Markers()

	// if this is the first marker create the genesis sequence and index
	if !referencedMarkersExist {
		referencedMarkers = NewMarkers(&Marker{sequenceID: 0, index: 0})
		referencedSequences = map[SequenceID]types.Empty{0: types.Void}
	}

	cachedSequence, newSequenceCreated := m.fetchSequence(referencedMarkers, rankOfReferencedSequences, sequenceAlias)
	if newSequenceCreated {
		cachedSequence.Consume(func(sequence *Sequence) {
			inheritedStructureDetails.IsPastMarker = true
			// sequence has just been created, so lowestIndex = highestIndex
			inheritedStructureDetails.PastMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: sequence.lowestIndex})

			m.registerReferencingMarker(referencedMarkers, NewMarker(sequence.id, sequence.lowestIndex))
		})
		return
	}

	cachedSequence.Consume(func(sequence *Sequence) {
		if _, fetchedSequenceReferenced := referencedSequences[sequence.ID()]; !fetchedSequenceReferenced {
			inheritedStructureDetails.PastMarkers = referencedMarkers
			return
		}

		if currentIndex, _ := referencedMarkers.Get(sequence.id); sequence.HighestIndex() == currentIndex && increaseIndexCallback(sequence.id, currentIndex) {
			if newIndex, increased := sequence.IncreaseHighestIndex(referencedMarkers); increased {
				inheritedStructureDetails.IsPastMarker = true
				inheritedStructureDetails.PastMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: newIndex})

				m.registerReferencingMarker(referencedMarkers, NewMarker(sequence.id, newIndex))

				return
			}
		}

		inheritedStructureDetails.PastMarkers = referencedMarkers
	})

	return inheritedStructureDetails, newSequenceCreated
}

// UpdateStructureDetails updates the StructureDetails of an existing node in the DAG by propagating new Markers of its
// children into its future Markers. It returns two boolean flags that indicate if the future Markers were updated and
// if the new Marker should be propagated further to the parents of the given node.
func (m *Manager) UpdateStructureDetails(structureDetailsToUpdate *StructureDetails, markerToInherit *Marker) (futureMarkersUpdated, inheritFutureMarkerFurther bool) {
	structureDetailsToUpdate.futureMarkersUpdateMutex.Lock()
	defer structureDetailsToUpdate.futureMarkersUpdateMutex.Unlock()

	// abort if future markers of structureDetailsToUpdate reference markerToInherit
	if m.markersReferenceMarkers(NewMarkers(markerToInherit), structureDetailsToUpdate.FutureMarkers, false) {
		return
	}

	structureDetailsToUpdate.FutureMarkers.Set(markerToInherit.sequenceID, markerToInherit.index)
	futureMarkersUpdated = true
	// stop propagating further if structureDetailsToUpdate is a marker
	inheritFutureMarkerFurther = !structureDetailsToUpdate.IsPastMarker

	return
}

// IsInPastCone checks if the earlier node is directly or indirectly referenced by the later node in the DAG.
func (m *Manager) IsInPastCone(earlierStructureDetails, laterStructureDetails *StructureDetails) (isInPastCone types.TriBool) {
	if earlierStructureDetails.Rank >= laterStructureDetails.Rank {
		return types.False
	}

	if earlierStructureDetails.PastMarkers.HighestIndex() > laterStructureDetails.PastMarkers.HighestIndex() {
		return types.False
	}

	if earlierStructureDetails.IsPastMarker {
		earlierMarker := earlierStructureDetails.PastMarkers.Marker()
		if earlierMarker == nil {
			panic("failed to retrieve Marker")
		}

		// If laterStructureDetails has a past marker in the same sequence of the earlier with a higher index
		// the earlier one is in its past cone.
		if laterIndex, sequenceExists := laterStructureDetails.PastMarkers.Get(earlierMarker.sequenceID); sequenceExists {
			if laterIndex >= earlierMarker.index {
				return types.True
			}

			return types.False
		}

		// If laterStructureDetails has no past marker in the same sequence of the earlier,
		// then just check the index
		if laterStructureDetails.PastMarkers.HighestIndex() <= earlierMarker.index {
			return types.False
		}
	}

	if laterStructureDetails.IsPastMarker {
		laterMarker := laterStructureDetails.PastMarkers.Marker()
		if laterMarker == nil {
			panic("failed to retrieve Marker")
		}

		// If earlierStructureDetails has a past marker in the same sequence of the later with a higher index or references the later,
		// the earlier one is definitely not in its past cone.
		if earlierIndex, sequenceExists := earlierStructureDetails.PastMarkers.Get(laterMarker.sequenceID); sequenceExists && earlierIndex >= laterMarker.index {
			return types.False
		}

		// If earlierStructureDetails has a future marker in the same sequence of the later with a higher index,
		// the earlier one is definitely not in its past cone.
		if earlierFutureIndex, earlierFutureIndexExists := earlierStructureDetails.FutureMarkers.Get(laterMarker.sequenceID); earlierFutureIndexExists && earlierFutureIndex > laterMarker.index {
			return types.False
		}

		// Iterate the future markers of laterStructureDetails and check if the earlier one has future markers in the same sequence,
		// if yes, then make sure the index is smaller than the one of laterStructureDetails.
		if laterStructureDetails.FutureMarkers.Size() != 0 && !laterStructureDetails.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			earlierIndex, similarSequenceExists := earlierStructureDetails.FutureMarkers.Get(sequenceID)
			return !similarSequenceExists || earlierIndex < laterIndex
		}) {
			return types.False
		}

		if earlierStructureDetails.PastMarkers.HighestIndex() >= laterMarker.index {
			return types.False
		}
	}

	// If the two messages has the same past marker, then the earlier one is not in the later one's past cone.
	if earlierStructureDetails.PastMarkers.HighestIndex() == laterStructureDetails.PastMarkers.HighestIndex() {
		if !earlierStructureDetails.PastMarkers.ForEach(func(sequenceID SequenceID, earlierIndex Index) bool {
			if earlierIndex == earlierStructureDetails.PastMarkers.HighestIndex() {
				laterIndex, sequenceExists := laterStructureDetails.PastMarkers.Get(sequenceID)
				return sequenceExists && laterIndex == earlierIndex
			}

			return true
		}) {
			return types.False
		}
	}

	if earlierStructureDetails.FutureMarkers.Size() != 0 && m.markersReferenceMarkers(laterStructureDetails.PastMarkers, earlierStructureDetails.FutureMarkers, false) {
		return types.True
	}

	if !m.markersReferenceMarkers(laterStructureDetails.PastMarkers, earlierStructureDetails.PastMarkers, false) {
		return types.False
	}

	if earlierStructureDetails.FutureMarkers.Size() != 0 && m.markersReferenceMarkers(earlierStructureDetails.FutureMarkers, laterStructureDetails.PastMarkers, true) {
		return types.Maybe
	}

	if earlierStructureDetails.FutureMarkers.Size() == 0 && laterStructureDetails.FutureMarkers.Size() == 0 {
		return types.Maybe
	}

	return types.False
}

// Sequence retrieves a Sequence from the object storage.
func (m *Manager) Sequence(sequenceID SequenceID) *CachedSequence {
	return &CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}
}

// SequenceFromAlias returns a Sequence from the given SequenceAlias.
func (m *Manager) SequenceFromAlias(sequenceAlias SequenceAlias) (cachedSequence *CachedSequence, exists bool) {
	exists = (&CachedSequenceAliasMapping{CachedObject: m.sequenceAliasMappingStore.Load(sequenceAlias.Bytes())}).Consume(func(sequenceAliasMapping *SequenceAliasMapping) {
		cachedSequence = m.Sequence(sequenceAliasMapping.SequenceID())
	})

	return
}

// RegisterSequenceAlias adds a mapping from a SequenceAlias to a Sequence.
func (m *Manager) RegisterSequenceAlias(sequenceAlias SequenceAlias, sequenceID SequenceID) {
	if cachedObject, stored := m.sequenceAliasMappingStore.StoreIfAbsent(&SequenceAliasMapping{
		sequenceAlias: sequenceAlias,
		sequenceID:    sequenceID,
	}); stored {
		cachedObject.Release()
	}
}

// UnregisterSequenceAlias removes the mapping of the given SequenceAlias to its corresponding Sequence.
func (m *Manager) UnregisterSequenceAlias(sequenceAlias SequenceAlias) {
	m.sequenceAliasMappingStore.Delete(sequenceAlias.Bytes())
}

// Shutdown shuts down the Manager and persists its state.
func (m *Manager) Shutdown() {
	m.shutdownOnce.Do(func() {
		if err := m.store.Set(kvstore.Key("sequenceIDCounter"), m.sequenceIDCounter.Bytes()); err != nil {
			panic(err)
		}

		m.sequenceStore.Shutdown()
		m.sequenceAliasMappingStore.Shutdown()
	})
}

// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) normalizeMarkers(markers *Markers) (normalizedMarkersByRank *markersByRank, referencedSequences SequenceIDs) {
	rankOfSequencesCache := make(map[SequenceID]uint64)

	normalizedMarkersByRank = newMarkersByRank()
	referencedSequences = make(SequenceIDs)
	// group markers with same sequence rank
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		referencedSequences[sequenceID] = types.Void
		normalizedMarkersByRank.Add(m.rankOfSequence(sequenceID, rankOfSequencesCache), sequenceID, index)

		return true
	})
	markersToIterate := normalizedMarkersByRank.Clone()

	// iterate from highest sequence rank to lowest
	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkersByRank.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.Markers(currentRank)
		if !rankExists {
			continue
		}

		// for each marker from the current sequence rank check if we can remove a marker in normalizedMarkersByRank,
		// and add the parent markers to markersToIterate if necessary
		if !markersByRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			if currentRank <= normalizedMarkersByRank.LowestRank() {
				return false
			}

			if !m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
				// for each of the parentMarkers of this particular index
				sequence.ReferencedMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					rankOfReferencedSequence := m.rankOfSequence(referencedSequenceID, rankOfSequencesCache)
					// check whether there is a marker in normalizedMarkersByRank that is from the same sequence
					if index, indexExists := normalizedMarkersByRank.Index(rankOfReferencedSequence, referencedSequenceID); indexExists {
						if referencedIndex >= index {
							// this referencedParentMarker is from the same sequence as a marker in the list but with higher index - hence remove the index from the Marker list
							normalizedMarkersByRank.Delete(rankOfReferencedSequence, referencedSequenceID)

							// if rankOfReferencedSequence is already the lowest rank of the original markers list,
							// no need to add it since parents of the referencedMarker cannot delete any further elements from the list
							if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
								markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
							}
						}

						return true
					}

					// if rankOfReferencedSequence is already the lowest rank of the original markers list,
					// no need to add it since parents of the referencedMarker cannot delete any further elements from the list
					if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
						markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
					}

					return true
				})
			}) {
				panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
			}

			return true
		}) {
			return
		}
	}

	return normalizedMarkersByRank, referencedSequences
}

// markersReferenceMarkersOfSameSequence is an internal utility function that determines if the given markers reference
// each other as part of the same Sequence.
func (m *Manager) markersReferenceMarkersOfSameSequence(laterMarkers, earlierMarkers *Markers, requireBiggerMarkers bool) (sameSequenceFound, referenceFound bool) {
	sameSequenceFound = !laterMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
		earlierIndex, sequenceExists := earlierMarkers.Get(sequenceID)
		if !sequenceExists {
			return true
		}

		if requireBiggerMarkers {
			referenceFound = earlierIndex < laterIndex
		} else {
			referenceFound = earlierIndex <= laterIndex
		}

		return false
	})
	return
}

// markersReferenceMarkers is an internal utility function that returns true if the later Markers reference the earlier
// Markers. If requireBiggerMarkers is false then a Marker with an equal Index is considered to be a valid reference.
func (m *Manager) markersReferenceMarkers(laterMarkers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
	rankCache := make(map[SequenceID]uint64)
	futureMarkersByRank := newMarkersByRank()

	continueScanningForPotentialReferences := func(laterMarkers *Markers, requireBiggerMarkers bool) bool {
		// don't abort scanning but don't execute additional checks in our current path (we can not find matching
		// markers anymore)
		if requireBiggerMarkers && earlierMarkers.LowestIndex() >= laterMarkers.HighestIndex() {
			return true
		}
		if earlierMarkers.LowestIndex() > laterMarkers.HighestIndex() {
			return true
		}

		// abort scanning if we reached a matching sequence
		var referencedSequenceFound bool
		if referencedSequenceFound, result = m.markersReferenceMarkersOfSameSequence(laterMarkers, earlierMarkers, requireBiggerMarkers); referencedSequenceFound {
			return false
		}

		// queue parents for additional checks
		laterMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
			m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
				sequence.ReferencedMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					futureMarkersByRank.Add(m.rankOfSequence(referencedSequenceID, rankCache), referencedSequenceID, referencedIndex)
					return true
				})
			})
			return true
		})

		return true
	}

	if !continueScanningForPotentialReferences(laterMarkers, requireBiggerMarkers) {
		return
	}

	rankOfLowestSequence := uint64(1<<64 - 1)
	earlierMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		if rankOfSequence := m.rankOfSequence(sequenceID, rankCache); rankOfSequence < rankOfLowestSequence {
			rankOfLowestSequence = rankOfSequence
		}

		return true
	})

	for rank := futureMarkersByRank.HighestRank() + 1; rank > rankOfLowestSequence; rank-- {
		markersByRank, rankExists := futureMarkersByRank.Markers(rank - 1)
		if !rankExists {
			continue
		}

		if !continueScanningForPotentialReferences(markersByRank, false) {
			return
		}
	}

	return result
}

// registerReferencingMarker is an internal utility function that adds a referencing Marker to the internal data
// structure.
func (m *Manager) registerReferencingMarker(referencedMarkers *Markers, marker *Marker) {
	referencedMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
			sequence.AddReferencingMarker(index, marker)
		})

		return true
	})
}

// fetchSequence is an internal utility function that retrieves or creates the Sequence that represents the given
// parameters and returns it.
func (m *Manager) fetchSequence(referencedMarkers *Markers, rank uint64, sequenceAlias SequenceAlias) (cachedSequence *CachedSequence, isNew bool) {
	cachedSequenceAliasMapping := &CachedSequenceAliasMapping{CachedObject: m.sequenceAliasMappingStore.ComputeIfAbsent(sequenceAlias.Bytes(), func(key []byte) objectstorage.StorableObject {
		m.sequenceIDCounterMutex.Lock()
		sequence := NewSequence(m.sequenceIDCounter, referencedMarkers, rank+1)
		m.sequenceIDCounter++
		m.sequenceIDCounterMutex.Unlock()

		cachedSequence = &CachedSequence{CachedObject: m.sequenceStore.Store(sequence)}

		sequenceAliasMapping := &SequenceAliasMapping{
			sequenceAlias: sequenceAlias,
			sequenceID:    sequence.id,
		}

		sequenceAliasMapping.Persist()
		sequenceAliasMapping.SetModified()

		return sequenceAliasMapping
	})}

	if isNew = cachedSequence != nil; isNew {
		cachedSequenceAliasMapping.Release()
		return
	}

	cachedSequenceAliasMapping.Consume(func(sequenceAliasMapping *SequenceAliasMapping) {
		cachedSequence = m.Sequence(sequenceAliasMapping.SequenceID())
	})

	return
}

// rankOfSequence is an internal utility function that returns the rank of the given Sequence.
func (m *Manager) rankOfSequence(sequenceID SequenceID, ranksCache map[SequenceID]uint64) uint64 {
	if rank, rankKnown := ranksCache[sequenceID]; rankKnown {
		return rank
	}

	if !m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
		ranksCache[sequenceID] = sequence.rank
	}) {
		panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
	}

	return ranksCache[sequenceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
