package marker

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type Sequence struct {
	id              SequenceID
	parentSequences SequenceIDs
}

func SequenceFromObjectStorage(key []byte, data []byte) (markerSequence objectstorage.StorableObject, err error) {
	return
}
