package marker

import (
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixSequence defines the storage prefix for the Sequence.
	PrefixMarkerSequence byte = iota
	// PrefixMessageMetadata defines the storage prefix for message metadata.
	PrefixSequenceAlias
)

// Manager is the managing entity for the Marker related business logic. It is stateful and automatically stores its
// state in an underlying KVStore.
type Manager struct {
	store                  kvstore.KVStore
	sequenceStore          *objectstorage.ObjectStorage
	sequenceAliasStore     *objectstorage.ObjectStorage
	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex
	shutdownOnce           sync.Once
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(store kvstore.KVStore) (manager *Manager) {
	storedSequenceIDCounter, err := store.Get(kvstore.Key("sequenceIDCounter"))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}

	sequenceIDCounter := SequenceID(1)
	if storedSequenceIDCounter != nil {
		sequenceIDCounter, _, err = SequenceIDFromBytes(storedSequenceIDCounter)
		if err != nil {
			panic(err)
		}
	}

	manager = &Manager{
		store:              store,
		sequenceStore:      objectstorage.NewFactory(store, database.PrefixMarker).New(PrefixMarkerSequence, SequenceFromObjectStorage),
		sequenceAliasStore: objectstorage.NewFactory(store, database.PrefixMarker).New(PrefixSequenceAlias, SequenceFromObjectStorage),
		sequenceIDCounter:  sequenceIDCounter,
	}

	if cachedSequence, stored := manager.sequenceStore.StoreIfAbsent(NewSequence(SequenceID(0), NewMarkers(), 0)); stored {
		cachedSequence.Release()
	}

	return
}

// InheritStructureDetails takes the StructureDetails of the referenced parents and return the new StructureDetails of
// the node that wqs just added to the DAG. It automatically creates new Sequences and Markers if necessary and returns
// an additional flag that indicate if a new Sequence was created.
func (m *Manager) InheritStructureDetails(referencedStructureDetails []*StructureDetails, increaseMarkerCallback IncreaseIndexCallback, newSequenceAlias ...SequenceAlias) (inheritedStructureDetails *StructureDetails, newSequenceCreated bool) {
	inheritedStructureDetails = &StructureDetails{
		Rank:          0,
		IsPastMarker:  false,
		FutureMarkers: NewMarkers(),
	}

	mergedPastMarkers := NewMarkers()
	for _, referencedMarkerPair := range referencedStructureDetails {
		mergedPastMarkers.Merge(referencedMarkerPair.PastMarkers)
		if referencedMarkerPair.Rank > inheritedStructureDetails.Rank {
			inheritedStructureDetails.Rank = referencedMarkerPair.Rank
		}
	}
	inheritedStructureDetails.Rank++

	normalizedMarkers, normalizedSequences := m.normalizeMarkers(mergedPastMarkers)
	referencedMarkers, _ := normalizedMarkers.Markers()
	rank := normalizedMarkers.HighestRank()

	if len(normalizedSequences) == 0 {
		referencedMarkers = NewMarkers(&Marker{sequenceID: 0, index: 0})
		normalizedSequences = map[SequenceID]types.Empty{
			0: types.Void,
		}
		if len(newSequenceAlias) == 0 {
			newSequenceAlias = []SequenceAlias{NewSequenceAlias([]byte("MAIN_SEQUENCE"))}
		}
	}

	cachedSequence, newSequenceCreated := m.fetchSequence(normalizedSequences, referencedMarkers, rank, newSequenceAlias...)
	if newSequenceCreated {
		cachedSequence.Consume(func(sequence *Sequence) {
			inheritedStructureDetails.IsPastMarker = true
			inheritedStructureDetails.PastMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: sequence.lowestIndex})
		})
		return
	}

	if len(normalizedSequences) == 1 {
		cachedSequence.Consume(func(sequence *Sequence) {
			currentIndex, _ := referencedMarkers.Get(sequence.id)

			if sequence.HighestIndex() == currentIndex && increaseMarkerCallback(sequence.id, currentIndex) {
				if newIndex, increased := sequence.IncreaseHighestIndex(referencedMarkers); increased {
					inheritedStructureDetails.IsPastMarker = true
					inheritedStructureDetails.PastMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: newIndex})
					return
				}
			}

			inheritedStructureDetails.PastMarkers = referencedMarkers
		})
		return
	}

	cachedSequence.Release()
	inheritedStructureDetails.PastMarkers = referencedMarkers

	return
}

// UpdateStructureDetails updates the StructureDetails of an existing node in the DAG by propagating new Markers of its
// children into its future Markers. It returns two boolean flags that indicate if the future Markers were updated and
// if the new Marker should also be propagated further to the parents of the given node.
func (m *Manager) UpdateStructureDetails(structureDetailsToUpdate *StructureDetails, markerToInherit *Marker) (futureMarkersUpdated bool, inheritFutureMarkerFurther bool) {
	structureDetailsToUpdate.futureMarkersUpdateMutex.Lock()
	defer structureDetailsToUpdate.futureMarkersUpdateMutex.Unlock()

	if m.markersReferenceMarkers(NewMarkers(markerToInherit), structureDetailsToUpdate.FutureMarkers, false) {
		return
	}

	structureDetailsToUpdate.FutureMarkers.Set(markerToInherit.sequenceID, markerToInherit.index)
	futureMarkersUpdated = true
	inheritFutureMarkerFurther = !structureDetailsToUpdate.IsPastMarker

	return
}

// IsInPastCone checks if the earlier node is directly or indirectly referenced by the later node in the DAG.
func (m *Manager) IsInPastCone(earlierStructureDetails *StructureDetails, laterStructureDetails *StructureDetails) (isInPastCone TriBool) {
	if earlierStructureDetails.Rank >= laterStructureDetails.Rank {
		return False
	}

	if earlierStructureDetails.PastMarkers.HighestIndex() > laterStructureDetails.PastMarkers.HighestIndex() {
		return False
	}

	if earlierStructureDetails.IsPastMarker {
		earlierMarker := earlierStructureDetails.PastMarkers.FirstMarker()
		if earlierMarker == nil {
			panic("failed to retrieve Marker")
		}

		if laterIndex, sequenceExists := laterStructureDetails.PastMarkers.Get(earlierMarker.sequenceID); sequenceExists {
			if laterIndex >= earlierMarker.index {
				return True
			}

			return False
		}

		if laterStructureDetails.PastMarkers.HighestIndex() <= earlierMarker.index {
			return False
		}
	}

	if laterStructureDetails.IsPastMarker {
		laterMarker := laterStructureDetails.PastMarkers.FirstMarker()
		if laterMarker == nil {
			panic("failed to retrieve Marker")
		}

		if earlierIndex, sequenceExists := earlierStructureDetails.PastMarkers.Get(laterMarker.sequenceID); sequenceExists && earlierIndex >= laterMarker.index {
			return False
		}

		if earlierFutureIndex, earlierFutureIndexExists := earlierStructureDetails.FutureMarkers.Get(laterMarker.sequenceID); earlierFutureIndexExists && earlierFutureIndex > laterMarker.index {
			return False
		}

		if !laterStructureDetails.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			earlierIndex, similarSequenceExists := earlierStructureDetails.FutureMarkers.Get(sequenceID)
			return !similarSequenceExists || earlierIndex < laterIndex
		}) {
			return False
		}

		if earlierStructureDetails.PastMarkers.HighestIndex() >= laterMarker.index {
			return False
		}
	}

	if earlierStructureDetails.PastMarkers.HighestIndex() == laterStructureDetails.PastMarkers.HighestIndex() {
		if !earlierStructureDetails.PastMarkers.ForEach(func(sequenceID SequenceID, earlierIndex Index) bool {
			if earlierIndex == earlierStructureDetails.PastMarkers.HighestIndex() {
				laterIndex, sequenceExists := laterStructureDetails.PastMarkers.Get(sequenceID)
				return sequenceExists && laterIndex == earlierIndex
			}

			return true
		}) {
			return False
		}
	}

	if m.markersReferenceMarkers(laterStructureDetails.PastMarkers, earlierStructureDetails.FutureMarkers, false) {
		return True
	}

	if !m.markersReferenceMarkers(laterStructureDetails.PastMarkers, earlierStructureDetails.PastMarkers, false) {
		return False
	}

	if m.markersReferenceMarkers(earlierStructureDetails.FutureMarkers, laterStructureDetails.PastMarkers, true) {
		return Maybe
	}

	return False
}

// Shutdown shuts down the Manager and persists its state.
func (m *Manager) Shutdown() {
	m.shutdownOnce.Do(func() {
		if err := m.store.Set(kvstore.Key("sequenceIDCounter"), m.sequenceIDCounter.Bytes()); err != nil {
			panic(err)
		}

		m.sequenceStore.Shutdown()
		m.sequenceAliasStore.Shutdown()
	})
}

// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) normalizeMarkers(markers *Markers) (normalizedMarkersByRank *MarkersByRank, normalizedSequences SequenceIDs) {
	rankOfSequencesCache := make(map[SequenceID]uint64)

	normalizedMarkersByRank = NewMarkersByRank()
	normalizedSequences = make(SequenceIDs)
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		normalizedSequences[sequenceID] = types.Void
		normalizedMarkersByRank.Add(m.rankOfSequence(sequenceID, rankOfSequencesCache), sequenceID, index)

		return true
	})
	markersToIterate := normalizedMarkersByRank.Clone()

	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkersByRank.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.Markers(currentRank)
		if !rankExists {
			continue
		}

		if !markersByRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			if currentRank <= normalizedMarkersByRank.LowestRank() {
				return false
			}

			if !(&CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}).Consume(func(sequence *Sequence) {
				sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					delete(normalizedSequences, referencedSequenceID)

					rankOfReferencedSequence := m.rankOfSequence(referencedSequenceID, rankOfSequencesCache)
					if index, indexExists := normalizedMarkersByRank.Index(rankOfReferencedSequence, referencedSequenceID); indexExists {
						if referencedIndex >= index {
							normalizedMarkersByRank.Delete(rankOfReferencedSequence, referencedSequenceID)

							if rankOfReferencedSequence > normalizedMarkersByRank.LowestRank() {
								markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
							}
						}

						return true
					}

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

	return
}

// markersReferenceMarkersOfSameSequence is an internal utility function that determines if a the given markers
// reference each other as part of the same Sequence.
func (m *Manager) markersReferenceMarkersOfSameSequence(laterMarkers *Markers, earlierMarkers *Markers, requireBiggerMarkers bool) (sameSequenceFound bool, referenceFound bool) {
	laterMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
		var earlierIndex Index
		if earlierIndex, sameSequenceFound = earlierMarkers.Get(sequenceID); sameSequenceFound {
			if requireBiggerMarkers {
				referenceFound = earlierIndex < laterIndex
			} else {
				referenceFound = earlierIndex <= laterIndex
			}

			return false
		}

		return true
	})

	return
}

// markersReferenceMarkers returns true if the later Markers reference the earlier Markers. The Markers have to be
// different (i.e. at least one Index higher) if requireBiggerMarkers is true.
func (m *Manager) markersReferenceMarkers(laterMarkers *Markers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
	rankCache := make(map[SequenceID]uint64)
	futureMarkersByRank := NewMarkersByRank()

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
			(&CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}).Consume(func(sequence *Sequence) {
				sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
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

	return
}

// fetchSequence is an internal utility function that retrieves or creates the Sequence that represents the given
// parameters and returns it.
func (m *Manager) fetchSequence(parentSequences SequenceIDs, referencedMarkers *Markers, rank uint64, newSequenceAlias ...SequenceAlias) (cachedSequence *CachedSequence, isNew bool) {
	if len(parentSequences) == 1 && len(newSequenceAlias) == 0 {
		for sequenceID := range parentSequences {
			cachedSequence = &CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}
			return
		}
	}

	sequenceAlias := parentSequences.Alias()
	if len(newSequenceAlias) >= 1 {
		sequenceAlias = sequenceAlias.Merge(newSequenceAlias[0])
	}

	cachedSequenceAliasMapping := &CachedSequenceAliasMapping{CachedObject: m.sequenceAliasStore.ComputeIfAbsent(sequenceAlias.Bytes(), func(key []byte) objectstorage.StorableObject {
		m.sequenceIDCounterMutex.Lock()
		sequence := NewSequence(m.sequenceIDCounter, referencedMarkers, rank+1)
		m.sequenceIDCounter++
		m.sequenceIDCounterMutex.Unlock()

		cachedSequence = &CachedSequence{CachedObject: m.sequenceStore.Store(sequence)}

		return &SequenceAliasMapping{
			sequenceAlias: sequenceAlias,
			sequenceID:    sequence.id,
		}
	})}

	if isNew = cachedSequence != nil; isNew {
		cachedSequenceAliasMapping.Release()
		return
	}

	cachedSequenceAliasMapping.Consume(func(aggregatedSequencesIDMapping *SequenceAliasMapping) {
		cachedSequence = &CachedSequence{CachedObject: m.sequenceStore.Load(aggregatedSequencesIDMapping.SequenceID().Bytes())}
	})

	return
}

// rankOfSequence is an internal utility function that returns the rank of the given Sequence.
func (m *Manager) rankOfSequence(sequenceID SequenceID, ranksCache map[SequenceID]uint64) uint64 {
	if rank, rankKnown := ranksCache[sequenceID]; rankKnown {
		return rank
	}

	if !(&CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}).Consume(func(sequence *Sequence) {
		ranksCache[sequenceID] = sequence.rank
	}) {
		panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
	}

	return ranksCache[sequenceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
