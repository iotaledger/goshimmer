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

// MergeMarkers takes multiple Markers and merges them into a single set of Markers by using the higher Index in case
// of ambiguous Indexes per SequenceID. It can be used to combine multiple Markers of multiple parents into a single
// object before handing it in to the other methods of the Manager.
func (m *Manager) MergeMarkers(markersToMerge ...Markers) (mergedMarkers Markers) {
	mergedMarkers = NewMarkers()
	for _, markers := range markersToMerge {
		for sequenceID, index := range markers {
			mergedMarkers.Set(sequenceID, index)
		}
	}

	return
}

// NormalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set). In addition,
// the method returns all SequenceIDs of the Markers that were not referenced by any of the Markers (the tips of the
// Sequence DAG) and the rank of the Sequence that is furthest away from the root.
func (m *Manager) NormalizeMarkers(markers Markers) (normalizedMarkers Markers, normalizedSequences SequenceIDs, rank uint64) {
	rankOfSequences := make(map[SequenceID]uint64)
	rankOfSequence := func(sequenceID SequenceID) uint64 {
		if rank, rankKnown := rankOfSequences[sequenceID]; rankKnown {
			return rank
		}

		if !m.sequence(sequenceID).Consume(func(sequence *Sequence) {
			rankOfSequences[sequenceID] = sequence.rank
		}) {
			panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
		}

		return rankOfSequences[sequenceID]
	}

	normalizedSequences = make(SequenceIDs)
	normalizedMarkerCandidates := NewMarkersByRank()
	for sequenceID, index := range markers {
		normalizedSequences[sequenceID] = types.Void
		normalizedMarkerCandidates.Add(rankOfSequence(sequenceID), sequenceID, index)
	}
	markersToIterate := normalizedMarkerCandidates.Clone()

	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkerCandidates.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.Markers(currentRank)
		if !rankExists {
			continue
		}

		for sequenceID, index := range markersByRank {
			if currentRank <= normalizedMarkerCandidates.LowestRank() {
				normalizedMarkers, _ = normalizedMarkerCandidates.Markers()
				rank = normalizedMarkerCandidates.HighestRank()
				return
			}

			m.sequence(sequenceID).Consume(func(sequence *Sequence) {
				for referencedSequenceID, referencedIndex := range sequence.HighestReferencedParentMarkers(index) {
					delete(normalizedSequences, referencedSequenceID)

					rankOfReferencedSequence := rankOfSequence(referencedSequenceID)
					if index, indexExists := normalizedMarkerCandidates.Index(rankOfReferencedSequence, referencedSequenceID); indexExists {
						if referencedIndex >= index {
							normalizedMarkerCandidates.Delete(rankOfReferencedSequence, referencedSequenceID)

							if rankOfReferencedSequence > normalizedMarkerCandidates.LowestRank() {
								markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
							}
						}

						continue
					}

					if rankOfReferencedSequence > normalizedMarkerCandidates.LowestRank() {
						markersToIterate.Add(rankOfReferencedSequence, referencedSequenceID, referencedIndex)
					}
				}
			})
		}
	}

	normalizedMarkers, _ = normalizedMarkerCandidates.Markers()
	rank = normalizedMarkerCandidates.HighestRank()
	return
}

// InheritMarkers takes the result of the NormalizeMarkers method and determines the resulting markers that should be
// inherited to the a node in the DAG. It automatically creates new Sequences and Markers if necessary and returns two
// additional flags that indicate if either a new Sequence and or a new Marker where created.
func (m *Manager) InheritMarkers(normalizedMarkers Markers, normalizedSequences SequenceIDs, rank uint64, newSequenceAlias ...SequenceAlias) (inheritedMarkers Markers, newSequence bool, newMarker bool) {
	if len(normalizedSequences) == 0 {
		normalizedSequences[SequenceID(0)] = types.Void
	}

	cachedSequence, newSequence := m.fetchSequence(normalizedSequences, normalizedMarkers, rank, newSequenceAlias...)
	if newSequence {
		cachedSequence.Consume(func(sequence *Sequence) {
			inheritedMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: sequence.lowestIndex})
			newMarker = true
		})
		return
	}

	if len(normalizedSequences) == 1 {
		cachedSequence.Consume(func(sequence *Sequence) {
			if sequence.HighestIndex() == normalizedMarkers[sequence.id] {
				newIndex, increased := sequence.IncreaseHighestIndex(normalizedMarkers[sequence.id])
				if increased {
					if len(normalizedMarkers) > 1 {
						delete(normalizedMarkers, sequence.id)
						sequence.parentReferences.AddReferences(normalizedMarkers, newIndex)
					}

					inheritedMarkers = NewMarkers(&Marker{sequenceID: sequence.id, index: newIndex})
					newMarker = true
					return
				}
			}

			inheritedMarkers = normalizedMarkers
		})
		return
	}

	cachedSequence.Release()
	inheritedMarkers = normalizedMarkers

	return
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
func (m *Manager) fetchSequence(parentSequences SequenceIDs, referencedMarkers Markers, rank uint64, newSequenceAlias ...SequenceAlias) (cachedSequence *CachedSequence, isNew bool) {
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

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
