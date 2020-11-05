package marker

import (
	"errors"
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
	return &CachedAggregatedSequencesIDMapping{CachedObject: s.aggregatedSequenceIDMappingStore.Get(id.Bytes())}
}

func (s *SequenceManager) Sequence(parentSequences SequenceIDs) (newSequence *Sequence) {
	s.sequenceIDCounterMutex.Lock()
	defer s.sequenceIDCounterMutex.Unlock()

	newSequence = &Sequence{
		id:              s.sequenceIDCounter,
		parentSequences: parentSequences,
	}
	s.sequenceIDCounter++

	return
}
