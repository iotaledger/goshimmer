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
	sequenceStore                    *objectstorage.ObjectStorage
	aggregatedSequenceIDMappingStore *objectstorage.ObjectStorage

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

func (s *SequenceManager) AggregatedSequenceIDMapping(id AggregatedSequencesID) *CachedAggregatedSequencesIDMapping {
	return &CachedAggregatedSequencesIDMapping{CachedObject: s.aggregatedSequenceIDMappingStore.Load(id.Bytes())}
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
}

func (s *SequenceManager) Sequence(sequenceID SequenceID) *CachedSequence {
	return &CachedSequence{CachedObject: s.sequenceStore.Load(sequenceID.Bytes())}
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
