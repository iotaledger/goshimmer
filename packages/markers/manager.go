package markers

import (
	"fmt"
	"math"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/database"
)

// region Manager //////////////////////////////////////////////////////////////////////////////////////////////////////

// Manager is the managing entity for the Marker related business logic. It is stateful and automatically stores its
// state in an underlying KVStore.
type Manager struct {
	store                  kvstore.KVStore
	sequenceStore          *objectstorage.ObjectStorage
	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex
	shutdownOnce           sync.Once
}

// NewManager is the constructor of the Manager that takes a KVStore to persist its state.
func NewManager(store kvstore.KVStore, cacheProvider *database.CacheTimeProvider) (newManager *Manager) {
	sequenceIDCounter := SequenceID(1)
	if storedSequenceIDCounter, err := store.Get(kvstore.Key("sequenceIDCounter")); err != nil && !errors.Is(err, kvstore.ErrKeyNotFound) {
		panic(err)
	} else if storedSequenceIDCounter != nil {
		if sequenceIDCounter, _, err = SequenceIDFromBytes(storedSequenceIDCounter); err != nil {
			panic(err)
		}
	}

	options := buildObjectStorageOptions(cacheProvider)
	osFactory := objectstorage.NewFactory(store, database.PrefixMarkers)
	newManager = &Manager{
		store:             store,
		sequenceStore:     osFactory.New(PrefixSequence, SequenceFromObjectStorage, options.objectStorageOptions...),
		sequenceIDCounter: sequenceIDCounter,
	}

	if cachedSequence, stored := newManager.sequenceStore.StoreIfAbsent(NewSequence(0, NewMarkers(), 0)); stored {
		cachedSequence.Release()
	}

	return
}

// InheritStructureDetails takes the StructureDetails of the referenced parents and returns new StructureDetails for the
// message that was just added to the DAG. It automatically creates a new Sequence and Index if necessary and returns an
// additional flag that indicates if a new Sequence was created.
// InheritStructureDetails inherits the structure details of the given parent StructureDetails.
func (m *Manager) InheritStructureDetails(referencedStructureDetails []*StructureDetails, increaseIndexCallback IncreaseIndexCallback) (inheritedStructureDetails *StructureDetails) {
	inheritedStructureDetails = m.mergeParentStructureDetails(referencedStructureDetails)

	normalizedMarkers, highestRankOfReferencedSequences := m.normalizeMarkers(inheritedStructureDetails.PastMarkers)
	if normalizedMarkers.Size() == 0 {
		normalizedMarkers = NewMarkers(&Marker{sequenceID: 0, index: 0})
	}
	inheritedStructureDetails.PastMarkers = normalizedMarkers

	assignedMarker, sequenceExtended := m.extendReferencedSequences(inheritedStructureDetails.PastMarkers, inheritedStructureDetails.PastMarkerGap, highestRankOfReferencedSequences, increaseIndexCallback)
	if sequenceExtended {
		inheritedStructureDetails.IsPastMarker = true
		inheritedStructureDetails.PastMarkerGap = 0
		inheritedStructureDetails.PastMarkers = NewMarkers(assignedMarker)
		fmt.Println(inheritedStructureDetails.PastMarkers)

		return
	}

	return inheritedStructureDetails
}

func (m *Manager) mergeParentStructureDetails(referencedStructureDetails []*StructureDetails) (mergedStructureDetails *StructureDetails) {
	// merge parent's pastMarkers
	mergedStructureDetails = &StructureDetails{
		PastMarkers:   NewMarkers(),
		FutureMarkers: NewMarkers(),
		PastMarkerGap: math.MaxUint64,
	}
	for _, referencedMarkerPair := range referencedStructureDetails {
		mergedStructureDetails.PastMarkers.Merge(referencedMarkerPair.PastMarkers)
		// update highest past marker gap
		if referencedMarkerPair.PastMarkerGap < mergedStructureDetails.PastMarkerGap {
			mergedStructureDetails.PastMarkerGap = referencedMarkerPair.PastMarkerGap
		}
		// update the highest rank
		if referencedMarkerPair.Rank > mergedStructureDetails.Rank {
			mergedStructureDetails.Rank = referencedMarkerPair.Rank
		}
	}
	// past marker gap for this message is set to the highest past marker gap of parents + 1
	mergedStructureDetails.PastMarkerGap++
	// rank for this message is set to the highest rank of parents + 1
	mergedStructureDetails.Rank++

	return mergedStructureDetails
}

// UpdateStructureDetails updates the StructureDetails of an existing node in the DAG by propagating new Markers of its
// children into its future Markers. It returns two boolean flags that indicate if the future Markers were updated and
// if the new Marker should be propagated further to the parents of the given node.
func (m *Manager) UpdateStructureDetails(structureDetailsToUpdate *StructureDetails, markerToInherit *Marker) (futureMarkersUpdated, inheritFutureMarkerFurther bool) {
	structureDetailsToUpdate.futureMarkersUpdateMutex.Lock()
	defer structureDetailsToUpdate.futureMarkersUpdateMutex.Unlock()

	// abort if future structureDetails of structureDetailsToUpdate reference markerToInherit
	if m.markersReferenceMarkers(NewMarkers(markerToInherit), structureDetailsToUpdate.FutureMarkers, false) {
		return
	}

	if structureDetailsToUpdate.FutureMarkers.Size() == 0 {
		structureDetailsToUpdate.PastMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
			m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
				sequence.decreaseVerticesWithoutFutureMarker()
			})

			return true
		})
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

		// Iterate the future structureDetails of laterStructureDetails and check if the earlier one has future structureDetails in the same sequence,
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

// Shutdown shuts down the Manager and persists its state.
func (m *Manager) Shutdown() {
	m.shutdownOnce.Do(func() {
		if err := m.store.Set(kvstore.Key("sequenceIDCounter"), m.sequenceIDCounter.Bytes()); err != nil {
			panic(err)
		}

		m.sequenceStore.Shutdown()
	})
}

// normalizeMarkers takes a set of Markers and removes each Marker that is already referenced by another Marker in the
// same set (the remaining Markers are the "most special" Markers that reference all Markers in the set grouped by the
// rank of their corresponding Sequence). In addition, the method returns all SequenceIDs of the Markers that were not
// referenced by any of the Markers (the tips of the Sequence DAG).
func (m *Manager) normalizeMarkers(markers *Markers) (normalizedMarkers *Markers, highestSequenceRank uint64) {
	normalizedMarkers = markers.Clone()

	rankCache := make(map[SequenceID]uint64)
	normalizeWalker := walker.New()
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		normalizeWalker.Push(NewMarker(sequenceID, index))

		return true
	})

	lowestRankOfMarkers := m.rankOfLowestSequence(markers, rankCache)

	seenMarkers := NewMarkers()
	for i := 0; normalizeWalker.HasNext(); i++ {
		currentMarker := normalizeWalker.Next().(*Marker)

		sequenceRank := m.rankOfSequence(currentMarker.SequenceID(), rankCache)
		if i < markers.Size() {
			if sequenceRank > highestSequenceRank {
				highestSequenceRank = sequenceRank
			}
		} else {
			if added, updated := seenMarkers.Set(currentMarker.SequenceID(), currentMarker.Index()); !added && !updated {
				continue
			}

			if sequenceRank < lowestRankOfMarkers {
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

		if !m.Sequence(currentMarker.SequenceID()).Consume(func(sequence *Sequence) {
			sequence.ReferencedMarkers(currentMarker.Index()).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
				normalizeWalker.Push(NewMarker(referencedSequenceID, referencedIndex))

				return true
			})
		}) {
			panic(fmt.Sprintf("failed to load Sequence with %s", currentMarker.SequenceID()))
		}
	}

	return normalizedMarkers, highestSequenceRank
}

// markersReferenceMarkers is an internal utility function that returns true if the later Markers reference the earlier
// Markers. If requireBiggerMarkers is false then a Marker with an equal Index is considered to be a valid reference.
func (m *Manager) markersReferenceMarkers(laterMarkers, earlierMarkers *Markers, requireBiggerMarkers bool) (result bool) {
	referenceWalker := walker.New()
	laterMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		referenceWalker.Push(NewMarker(sequenceID, index))

		return true
	})

	seenMarkers := NewMarkers()
	rankCache := make(map[SequenceID]uint64)
	lowestRankOfEarlierPastMarkers := m.rankOfLowestSequence(earlierMarkers, rankCache)
	for i := 0; referenceWalker.HasNext(); i++ {
		marker := referenceWalker.Next().(*Marker)
		if m.rankOfSequence(marker.SequenceID(), rankCache) < lowestRankOfEarlierPastMarkers {
			continue
		}

		result = m.markerReferencesMarkers(marker, earlierMarkers, i < laterMarkers.Size() && requireBiggerMarkers, seenMarkers, referenceWalker)
	}

	return result
}

// markerReferencesMarkers is used to recursively check if the given Marker and its parents have an overlap with the
// given Markers.
func (m *Manager) markerReferencesMarkers(marker *Marker, markers *Markers, requireBiggerMarkers bool, seenMarkers *Markers, w *walker.Walker) bool {
	if added, updated := seenMarkers.Set(marker.SequenceID(), marker.Index()); !added && !updated {
		return false
	}

	if markers.LowestIndex() > marker.Index() || (requireBiggerMarkers && markers.LowestIndex() >= marker.Index()) {
		return false
	}

	if earlierIndex, sequenceExists := markers.Get(marker.SequenceID()); sequenceExists {
		if (requireBiggerMarkers && earlierIndex < marker.Index()) || (!requireBiggerMarkers && earlierIndex <= marker.Index()) {
			w.StopWalk()

			return true
		}

		return false
	}

	m.Sequence(marker.SequenceID()).Consume(func(sequence *Sequence) {
		sequence.ReferencedMarkers(marker.Index()).ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
			w.Push(NewMarker(referencedSequenceID, referencedIndex))

			return true
		})
	})

	return false
}

func (m *Manager) rankOfLowestSequence(markers *Markers, rankCache map[SequenceID]uint64) (rankOfLowestSequence uint64) {
	rankOfLowestSequence = uint64(1<<64 - 1)
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		if rankOfSequence := m.rankOfSequence(sequenceID, rankCache); rankOfSequence < rankOfLowestSequence {
			rankOfLowestSequence = rankOfSequence
		}

		return true
	})

	return
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

// extendReferencedSequences is an internal utility function that retrieves or creates the Sequence that represents the given
// parameters and returns it.
func (m *Manager) extendReferencedSequences(referencedMarkers *Markers, pastMarkerGap, sequenceRank uint64, increaseIndexCallback IncreaseIndexCallback) (marker *Marker, extended bool) {
	if marker, extended = m.extendHighestAvailableSequence(referencedMarkers, increaseIndexCallback); extended {
		return
	}

	fmt.Println(referencedMarkers)

	// increase counters per sequence
	newSequenceRequired := false
	referencedMarkers.ForEach(func(sequenceID SequenceID, index Index) bool {
		m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
			sequence.increaseVerticesWithoutFutureMarker()
			if newSequenceRequired = sequence.newSequenceRequired(pastMarkerGap); !newSequenceRequired {
				return
			}
			fmt.Println("requireNewSequence", sequenceID, index)

			// create new sequence
			m.sequenceIDCounterMutex.Lock()
			newSequence := NewSequence(m.sequenceIDCounter, referencedMarkers, sequenceRank+1)
			newSequence.increaseVerticesWithoutFutureMarker()
			m.sequenceIDCounter++
			m.sequenceIDCounterMutex.Unlock()

			(&CachedSequence{CachedObject: m.sequenceStore.Store(newSequence)}).Release()

			marker = NewMarker(newSequence.id, newSequence.lowestIndex)
			fmt.Println(marker.SequenceID(), marker.Index())
		})

		return !newSequenceRequired
	})

	if newSequenceRequired {
		fmt.Println(marker.SequenceID(), marker.Index(), newSequenceRequired)
	}
	return marker, newSequenceRequired
}

func (m *Manager) extendHighestAvailableSequence(referencedMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (marker *Marker, extended bool) {
	referencedMarkers.ForEachSorted(func(sequenceID SequenceID, index Index) bool {
		m.Sequence(sequenceID).Consume(func(sequence *Sequence) {
			if newIndex, sequenceExtended := sequence.ExtendSequence(referencedMarkers, increaseIndexCallback); sequenceExtended {
				extended = sequenceExtended
				marker = NewMarker(sequenceID, newIndex)

				m.registerReferencingMarker(referencedMarkers, marker)
			}
		})

		return !extended
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
