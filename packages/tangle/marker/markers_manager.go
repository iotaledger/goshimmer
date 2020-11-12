package marker

import (
	"errors"
	"fmt"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/objectstorage"
)

type MarkersManager struct {
	sequenceStore      *objectstorage.ObjectStorage
	sequenceAliasStore *objectstorage.ObjectStorage

	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex
}

func NewMarkersManager(store kvstore.KVStore) *MarkersManager {
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

	return &MarkersManager{
		sequenceStore:     objectstorage.NewFactory(store, database.PrefixMessageLayer).New(tangle.PrefixMarkerSequence, SequenceFromObjectStorage),
		sequenceIDCounter: sequenceIDCounter,
	}
}

func (s *MarkersManager) InheritMarkers(referencedMarkers ...NormalizedMarkers) {
	normalizedMarkers, rank := s.NormalizeMarkers(referencedMarkers...)

	switch len(normalizedMarkers) {
	case 0:
	// create root chain
	case 1:
	// return a marker from the same chain
	default:
		// create a new Sequence
		s.RetrieveSequence(normalizedMarkers, rank)
	}

	return
}

func (s *MarkersManager) NormalizeMarkers(markers ...NormalizedMarkers) (normalizedMarkers NormalizedMarkers, rank uint64) {
	rankOfSequences := make(map[SequenceID]uint64)
	rankOfSequence := func(sequenceID SequenceID) uint64 {
		if rank, rankKnown := rankOfSequences[sequenceID]; rankKnown {
			return rank
		}

		if !s.Sequence(sequenceID).Consume(func(sequence *Sequence) {
			rankOfSequences[sequenceID] = sequence.rank
		}) {
			panic(fmt.Sprintf("failed to load Sequence with %s", sequenceID))
		}

		return rankOfSequences[sequenceID]
	}

	normalizedMarkerCandidates := NewNormalizedMarkersByRank()
	for _, marker := range markers {
		for sequenceID, index := range marker {
			normalizedMarkerCandidates.Add(rankOfSequence(sequenceID), sequenceID, index)
		}
	}
	markersToIterate := normalizedMarkerCandidates.Clone()

	for i := markersToIterate.HighestRank() + 1; i > normalizedMarkerCandidates.LowestRank(); i-- {
		currentRank := i - 1
		markersByRank, rankExists := markersToIterate.NormalizedMarkers(currentRank)
		if !rankExists {
			continue
		}

		for sequenceID, index := range markersByRank {
			s.Sequence(sequenceID).Consume(func(sequence *Sequence) {
				if currentRank <= normalizedMarkerCandidates.LowestRank() {
					normalizedMarkers, _ = normalizedMarkerCandidates.NormalizedMarkers()
					rank = normalizedMarkerCandidates.HighestRank()
					return
				}

				for referencedSequenceID, referencedIndex := range sequence.HighestReferencedParentMarkers(index) {
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

	normalizedMarkers, _ = normalizedMarkerCandidates.NormalizedMarkers()
	rank = normalizedMarkerCandidates.HighestRank()
	return
}

func (s *MarkersManager) NextSequenceID() (nextSequenceID SequenceID) {
	s.sequenceIDCounterMutex.Lock()
	defer s.sequenceIDCounterMutex.Unlock()

	nextSequenceID = s.sequenceIDCounter
	s.sequenceIDCounter++

	return
}

func (s *MarkersManager) RetrieveSequence(referencedMarkers NormalizedMarkers, rank uint64, optionalAlias ...SequenceAlias) (sequence *Sequence, sequenceCreated bool) {
	sequenceAlias := referencedMarkers.SequenceIDs().Alias()
	if len(optionalAlias) >= 1 {
		sequenceAlias = sequenceAlias.Merge(optionalAlias[0])
	}

	s.sequenceAliasStore.ComputeIfAbsent(sequenceAlias.Bytes(), func(key []byte) objectstorage.StorableObject {
		sequenceID := s.NextSequenceID()
		s.sequenceStore.Store(NewSequence(sequenceID, referencedMarkers, rank+1))

		return &SequenceAliasMapping{
			sequenceAlias: sequenceAlias,
			sequenceID:    sequenceID,
		}
	})

	return
}

func (s *MarkersManager) AggregatedSequence(optionalSequenceIDs ...SequenceID) *CachedSequence {
	var aggregatedSequenceIDs SequenceIDs
	switch len(optionalSequenceIDs) {
	case 0:
	}
	var sequenceToRetrieve SequenceID
	if len(optionalSequenceIDs) == 1 {
		sequenceToRetrieve = optionalSequenceIDs[0]
	}

	sequenceIDs := NewSequenceIDs(optionalSequenceIDs...)

	fmt.Println(sequenceIDs)
	fmt.Println(aggregatedSequenceIDs)
	fmt.Println(sequenceToRetrieve)

	return nil
}

func (s *MarkersManager) Sequence(sequenceID SequenceID) *CachedSequence {
	return &CachedSequence{CachedObject: s.sequenceStore.Load(sequenceID.Bytes())}
}

func (s *MarkersManager) SequenceAliasMapping(id SequenceAlias) *CachedSequenceAliasMapping {
	return &CachedSequenceAliasMapping{CachedObject: s.sequenceAliasStore.Load(id.Bytes())}
}
