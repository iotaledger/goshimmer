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

// NormalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) NormalizeMarkers(markers *Markers) (normalizedMarkersByRank *MarkersByRank, normalizedSequences SequenceIDs) {
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

// InheritPastMarkers takes the result of the NormalizeMarkers method and determines the resulting markers that should
// be inherited to the a node in the DAG. It automatically creates new Sequences and Markers if necessary and returns
// two additional flags that indicate if either a new Sequence and or a new Marker where created.
func (m *Manager) InheritPastMarkers(mergedPastMarkers *Markers, increaseMarkerCallback IncreaseMarkerCallback, newSequenceAlias ...SequenceAlias) (inheritedMarkers *Markers, newSequence bool, futureMarkerToPropagate *Marker) {
	normalizedMarkers, normalizedSequences := m.NormalizeMarkers(mergedPastMarkers)
	referencedMarkers, _ := normalizedMarkers.Markers()
	rank := normalizedMarkers.HighestRank()

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
				if newIndex, increased := sequence.IncreaseHighestIndex(currentIndex); increased {
					if referencedMarkers.Size() > 1 {
						referencedMarkers.Delete(sequence.id)

						sequence.parentReferences.AddReferences(referencedMarkers, newIndex)
					}

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
	rankCache := make(map[SequenceID]uint64)
	candidatesByRank := NewMarkersByRank()

	continueScanningForPotentialReferences := func(sequenceID SequenceID, index Index) bool {
		if existingIndex, sequenceExists := futureMarkers.Get(sequenceID); sequenceExists {
			return existingIndex > index
		}

		m.sequence(sequenceID).Consume(func(sequence *Sequence) {
			sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
				candidatesByRank.Add(m.rankOfSequence(referencedSequenceID, rankCache), referencedSequenceID, referencedIndex)

				return true
			})
		})

		return true
	}

	if !continueScanningForPotentialReferences(markerToInherit.sequenceID, markerToInherit.index) {
		return
	}

	rankOfLowestSequence := uint64(1<<64 - 1)
	futureMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		if rankOfSequence := m.rankOfSequence(sequenceID, rankCache); rankOfSequence < rankOfLowestSequence {
			rankOfLowestSequence = rankOfSequence
		}

		return true
	})

	for rank := candidatesByRank.HighestRank() + 1; rank > rankOfLowestSequence; rank-- {
		markersByRank, rankExists := candidatesByRank.Markers(rank - 1)
		if !rankExists {
			continue
		}

		if !markersByRank.ForEach(continueScanningForPotentialReferences) {
			return
		}
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

// CheckReference checks if the markers given by the first parameter are referenced by the
func (m *Manager) CheckReference(olderPastMarkers *Markers, olderFutureMarkers *Markers, laterPastMarkers *Markers, markers *Markers, referencedByMarkers *Markers) (referenced bool) {
	if olderPastMarkers.LowestIndex() > laterPastMarkers.HighestIndex() {
		return false
	}

	rankOfSequencesCache := make(map[SequenceID]uint64)

	referencedByMarkersByRank := NewMarkersByRank()
	referencedByMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		referencedByMarkersByRank.Add(m.rankOfSequence(sequenceID, rankOfSequencesCache), sequenceID, index)

		return true
	})

	for i := referencedByMarkersByRank.HighestRank() + 1; true; i-- {
		currentRank := i - 1
		markersByRank, rankExists := referencedByMarkersByRank.Markers(currentRank)
		if !rankExists {
			continue
		}

		if !markersByRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			if !m.sequence(sequenceID).Consume(func(sequence *Sequence) {
				sequence.HighestReferencedParentMarkers(index).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
					if index, exists := markers.Get(referencedSequenceID); exists && index <= referencedIndex {
						referenced = true
						return false
					}

					return true
				})
			}) {
				panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
			}

			if referenced {
				return false
			}

			return true
		}) {
			return
		}
	}

	return false
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
