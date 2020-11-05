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
	sequenceStore *objectstorage.ObjectStorage

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

func (m *SequenceManager) Sequence(parentSequences SequenceIDs) (newSequence *Sequence) {
	m.sequenceIDCounterMutex.Lock()
	defer m.sequenceIDCounterMutex.Unlock()

	newSequence = &Sequence{
		id:              m.sequenceIDCounter,
		parentSequences: parentSequences,
	}
	m.sequenceIDCounter++

	return
}
