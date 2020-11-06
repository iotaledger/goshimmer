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

type SequenceManager struct {
	sequenceStore      *objectstorage.ObjectStorage
	sequenceAliasStore *objectstorage.ObjectStorage

	sequenceIDCounter      SequenceID
	sequenceIDCounterMutex sync.Mutex
}

func NewSequenceManager(store kvstore.KVStore) *SequenceManager {
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

	return &SequenceManager{
		sequenceStore:     objectstorage.NewFactory(store, database.PrefixMessageLayer).New(tangle.PrefixMarkerSequence, SequenceFromObjectStorage),
		sequenceIDCounter: sequenceIDCounter,
	}
}

func (s *SequenceManager) NextSequenceID() (nextSequenceID SequenceID) {
	s.sequenceIDCounterMutex.Lock()
	defer s.sequenceIDCounterMutex.Unlock()

	nextSequenceID = s.sequenceIDCounter
	s.sequenceIDCounter++

	return
}

func (s *SequenceManager) SequenceFromAlias(alias SequenceAlias, referencedMarkers Markers) (sequence *Sequence, sequenceCreated bool) {
	s.sequenceAliasStore.ComputeIfAbsent(alias.Bytes(), func(key []byte) objectstorage.StorableObject {
		//newSequence := NewSequence(s.NextSequenceID())
		//s.sequenceStore.Store()

		return nil
	})

	return
}

func (s *SequenceManager) NormalizeMarkers(referencedMarkers Markers) (normalizedMarkers Markers, highestRank uint64) {
	normalizedMarkers = make(Markers, 0)

	sequencesByRank := make(map[uint64]map[SequenceID]*Sequence)
	referencedSequences := make(map[SequenceID]*Sequence)
	highestMarkers := make(map[SequenceID]Index)
	lowestRank := uint64(1)<<64 - 1
	for _, marker := range referencedMarkers {
		_, referencedSequenceAlreadyLoaded := referencedSequences[marker.sequenceID]
		if !referencedSequenceAlreadyLoaded {
			cachedSequence := s.Sequence(marker.sequenceID)
			defer cachedSequence.Release()

			sequence := cachedSequence.Unwrap()
			if sequence == nil {
				panic(fmt.Sprintf("Sequence belonging to Marker %s does not exist", marker))
			}

			if sequence.rank < lowestRank {
				lowestRank = sequence.rank
			}
			if sequence.rank > highestRank {
				highestRank = sequence.rank
			}

			sequencesByRankMap, mapExists := sequencesByRank[sequence.rank]
			if !mapExists {
				sequencesByRankMap = make(map[SequenceID]*Sequence)
				sequencesByRank[sequence.rank] = sequencesByRankMap
			}
			sequencesByRankMap[marker.sequenceID] = sequence
			referencedSequences[marker.sequenceID] = sequence
		}

		if previousMarker, previousMarkerExists := highestMarkers[marker.sequenceID]; !previousMarkerExists || marker.index > previousMarker {
			highestMarkers[marker.sequenceID] = marker.index
		}
	}

	if len(highestMarkers) == 0 {
		return
	}

	if len(highestMarkers) == 1 {
		for sequenceID, index := range highestMarkers {
			normalizedMarkers = append(normalizedMarkers, New(sequenceID, index))
		}

		return
	}

	for currentRank := highestRank; currentRank >= lowestRank; currentRank-- {

	}

	return
}

func (s *SequenceManager) InheritMarkers(referencedMarkers Markers) (inheritedMarkers Markers, newMarkerCreated bool) {
	referencedSequences := make(map[SequenceID]*Sequence)
	highestMarkers := make(map[SequenceID]Index)
	for _, marker := range referencedMarkers {
		cachedSequence := s.Sequence(marker.sequenceID)
		defer cachedSequence.Release()

		if referencedSequences[marker.sequenceID] = cachedSequence.Unwrap(); referencedSequences[marker.sequenceID] == nil {
			panic(fmt.Sprintf("Sequence belonging to inherited Marker does not exist: %s", marker.sequenceID))
		}

		if previousMarker, previousMarkerExists := highestMarkers[marker.sequenceID]; !previousMarkerExists || marker.index > previousMarker {
			highestMarkers[marker.sequenceID] = marker.index
		}
	}

	if len(highestMarkers) == 0 {
		// create initial marker chain (0)
	}

	if len(highestMarkers) == 1 {
		// inherit the highest marker
	}

	return
}

func (s *SequenceManager) AggregatedSequence(optionalSequenceIDs ...SequenceID) *CachedSequence {
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

func (s *SequenceManager) Sequence0r(parentSequences SequenceIDs) (newSequence *Sequence) {
	s.sequenceIDCounterMutex.Lock()
	defer s.sequenceIDCounterMutex.Unlock()

	newSequence = &Sequence{
		id:              s.sequenceIDCounter,
		parentSequences: parentSequences,
	}
	s.sequenceIDCounter++

	return
}

func (s *SequenceManager) Sequence(sequenceID SequenceID) *CachedSequence {
	return &CachedSequence{CachedObject: s.sequenceStore.Load(sequenceID.Bytes())}
}

func (s *SequenceManager) SequenceAliasMapping(id SequenceAlias) *CachedSequenceAliasMapping {
	return &CachedSequenceAliasMapping{CachedObject: s.sequenceAliasStore.Load(id.Bytes())}
}
