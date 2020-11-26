package marker

import (
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

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
func NewManager(store kvstore.KVStore) *Manager {
	storedSequenceIDCounter, err := store.Get(kvstore.Key("sequenceIDCounter"))
	if err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	}

	var sequenceIDCounter SequenceID
	if storedSequenceIDCounter != nil {
		sequenceIDCounter, _, err = SequenceIDFromBytes(storedSequenceIDCounter)
		if err != nil {
			panic(err)
		}
	}

	return &Manager{
		store:              store,
		sequenceStore:      objectstorage.NewFactory(store, database.PrefixMessageLayer).New(tangle.PrefixMarkerSequence, SequenceFromObjectStorage),
		sequenceAliasStore: objectstorage.NewFactory(store, database.PrefixMessageLayer).New(tangle.PrefixSequenceAlias, SequenceFromObjectStorage),
		sequenceIDCounter:  sequenceIDCounter,
	}
}

// InheritPastMarkers takes the result of the normalizeMarkers method and determines the resulting markers that should
// be inherited to the a node in the DAG. It automatically creates new Sequences and Markers if necessary and returns
// two additional flags that indicate if either a new Sequence and or a new Marker where created.
func (m *Manager) InheritPastMarkers(mergedPastMarkers *Markers, increaseMarkerCallback IncreaseMarkerCallback, newSequenceAlias ...SequenceAlias) (inheritedMarkers *Markers, newSequence bool, futureMarkerToPropagate *Marker) {
	normalizedMarkers, normalizedSequences := m.normalizeMarkers(mergedPastMarkers)
	referencedMarkers, _ := normalizedMarkers.Markers()
	rank := normalizedMarkers.HighestRank()

	if len(normalizedSequences) == 0 {
		normalizedSequences = map[SequenceID]types.Empty{
			0: types.Void,
		}
	}

	cachedSequence, newSequence := m.fetchSequence(normalizedSequences, referencedMarkers, rank, newSequenceAlias...)
	if newSequence {
		cachedSequence.Consume(func(sequence *Sequence) {
			futureMarkerToPropagate = &Marker{sequenceID: sequence.id, index: sequence.lowestIndex}
			inheritedMarkers = NewMarkers(futureMarkerToPropagate)
		})
		return
	}

	if len(normalizedSequences) == 1 {
		cachedSequence.Consume(func(sequence *Sequence) {
			currentIndex, _ := referencedMarkers.Get(sequence.id)

			if sequence.HighestIndex() == currentIndex && increaseMarkerCallback(sequence.id, currentIndex) {
				if newIndex, increased := sequence.IncreaseHighestIndex(referencedMarkers); increased {
					futureMarkerToPropagate = &Marker{sequenceID: sequence.id, index: newIndex}
					inheritedMarkers = NewMarkers(futureMarkerToPropagate)
					return
				}
			}

			inheritedMarkers = referencedMarkers
		})
		return
	}

	cachedSequence.Release()
	inheritedMarkers = referencedMarkers

	return
}

func (m *Manager) InheritFutureMarkers(futureMarkers *Markers, markerToInherit *Marker, messageIsMarker bool) (newFutureMarkers *Markers, futureMarkersUpdated bool, inheritFutureMarkerFurther bool) {
	if m.markersReferenceMarkers(NewMarkers(markerToInherit), futureMarkers, false) {
		return
	}

	newFutureMarkers = NewMarkers(markerToInherit)
	futureMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		newFutureMarkers.Set(sequenceID, index)

		return true
	})
	futureMarkersUpdated = true
	inheritFutureMarkerFurther = !messageIsMarker

	return
}

// IsInPastCone checks if the earlier Markers are directly or indirectly referenced by the later Markers.
func (m *Manager) IsInPastCone(earlierMarkers *MarkersPair, laterMarkers *MarkersPair) (referenced TriBool) {
	// fast check: if earlier Markers have larger highest Indexes they can't be in the past cone
	if earlierMarkers.PastMarkers.HighestIndex() > laterMarkers.PastMarkers.HighestIndex() {
		return False
	}

	// fast check: if earlier Marker is a past Marker and the later ones reference it we can return early
	if earlierMarkers.IsPastMarker {
		earlierMarker := earlierMarkers.PastMarkers.FirstMarker()
		if earlierMarker == nil {
			panic("failed to retrieve Marker")
		}

		if laterIndex, sequenceExists := laterMarkers.PastMarkers.Get(earlierMarker.sequenceID); sequenceExists {
			if laterIndex >= earlierMarker.index {
				return True
			}

			return False
		}

		if laterMarkers.PastMarkers.HighestIndex() <= earlierMarker.index {
			return False
		}
	}

	if laterMarkers.IsPastMarker {
		laterMarker := laterMarkers.PastMarkers.FirstMarker()
		if laterMarker == nil {
			panic("failed to retrieve Marker")
		}

		// if the earlier Marker inherited an Index of the same Sequence that is higher than the later we return false
		if earlierIndex, sequenceExists := earlierMarkers.PastMarkers.Get(laterMarker.sequenceID); sequenceExists && earlierIndex >= laterMarker.index {
			return False
		}

		// if the earlier Markers are referenced by a Marker of the same Sequence that is larger, we are not in the past cone
		if earlierFutureIndex, earlierFutureIndexExists := earlierMarkers.FutureMarkers.Get(laterMarker.sequenceID); earlierFutureIndexExists && earlierFutureIndex > laterMarker.index {
			return False
		}

		// if the earlier Markers were referenced by the same or a higher future Marker we are not in the past cone
		// (otherwise we would be the future marker)
		if !laterMarkers.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			earlierIndex, similarSequenceExists := earlierMarkers.FutureMarkers.Get(sequenceID)
			return !similarSequenceExists || earlierIndex < laterIndex
		}) {
			return False
		}

		if earlierMarkers.PastMarkers.HighestIndex() >= laterMarker.index {
			return False
		}
	}

	// if the highest Indexes of both past Markers are the same ...
	if earlierMarkers.PastMarkers.HighestIndex() == laterMarkers.PastMarkers.HighestIndex() {
		// ... then the later Markers should contain exact copies of all of the highest earlier Markers because parent
		// Markers get inherited and if they would have been captured by a new Marker in between then the highest
		// Indexes would no longer be the same
		if !earlierMarkers.PastMarkers.ForEach(func(sequenceID SequenceID, earlierIndex Index) bool {
			if earlierIndex == earlierMarkers.PastMarkers.HighestIndex() {
				laterIndex, sequenceExists := laterMarkers.PastMarkers.Get(sequenceID)
				return sequenceExists && laterIndex != earlierIndex
			}

			return true
		}) {
			return False
		}
	}

	if earlierMarkers.FutureMarkers.HighestIndex() == laterMarkers.FutureMarkers.HighestIndex() && false {
		// the earlier future markers need to contain all later ones because if there would be another marker in between that shadows them the later future Marker would have a higher index
		if !laterMarkers.FutureMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
			if laterIndex == laterMarkers.FutureMarkers.highestIndex {
				earlierIndex, sequenceExists := earlierMarkers.FutureMarkers.Get(sequenceID)
				return sequenceExists && earlierIndex == laterIndex
			}

			return true
		}) {
			return False
		}
	}

	// detailed check: earlier marker is referenced by something that the later one references
	if m.markersReferenceMarkers(laterMarkers.PastMarkers, earlierMarkers.FutureMarkers, false) {
		return True
	}

	// detailed check: the
	if m.markersReferenceMarkers(earlierMarkers.FutureMarkers, laterMarkers.PastMarkers, true) {
		return Maybe
	}

	return False
}

// Shutdown is the function that shuts down the Manager and persists its state.
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

			if !m.sequence(sequenceID).Consume(func(sequence *Sequence) {
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

func (m *Manager) markersReferenceMarkersOfSameSequence(laterMarkers *Markers, earlierMarkers *Markers, requireBiggerMarkers bool) (sameSequenceFound bool, referenceValid bool) {
	laterMarkers.ForEach(func(sequenceID SequenceID, laterIndex Index) bool {
		var earlierIndex Index
		if earlierIndex, sameSequenceFound = earlierMarkers.Get(sequenceID); sameSequenceFound {
			if requireBiggerMarkers {
				referenceValid = earlierIndex < laterIndex
			} else {
				referenceValid = earlierIndex <= laterIndex
			}

			return false
		}

		return true
	})

	return
}

func (m *Manager) markersReferenceMarkers(laterMarkers *Markers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
	rankCache := make(map[SequenceID]uint64)
	futureMarkersByRank := NewMarkersByRank()

	continueScanningForPotentialReferences := func(laterMarkers *Markers, requireBiggerMarkers bool) bool {
		// don't abort scanning but don't execute additional checks (we can never find matching markers anymore)
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
			m.sequence(sequenceID).Consume(func(sequence *Sequence) {
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
	switch len(parentSequences) {
	case 1:
		for sequenceID := range parentSequences {
			cachedSequence = m.sequence(sequenceID)
			return
		}
	}

	if len(parentSequences) == 0 {
		parentSequences = map[SequenceID]types.Empty{
			0: types.Void,
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
		cachedSequence = m.sequence(aggregatedSequencesIDMapping.SequenceID())
	})

	return
}

// sequence is an internal utility function that loads the given Sequence from the store.
func (m *Manager) sequence(sequenceID SequenceID) *CachedSequence {
	return &CachedSequence{CachedObject: m.sequenceStore.Load(sequenceID.Bytes())}
}

// rankOfSequence is an internal utility function that returns the rank of the given Sequence.
func (m *Manager) rankOfSequence(sequenceID SequenceID, ranksCache map[SequenceID]uint64) uint64 {
	if rank, rankKnown := ranksCache[sequenceID]; rankKnown {
		return rank
	}

	if !m.sequence(sequenceID).Consume(func(sequence *Sequence) {
		ranksCache[sequenceID] = sequence.rank
	}) {
		panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
	}

	return ranksCache[sequenceID]
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
