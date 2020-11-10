package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test(t *testing.T) {
	markerSequenceManager := NewSequenceManager(mapdb.NewMapDB())

	markerSequenceManager.sequenceStore.Store(NewSequence(0, Markers{}, 0))
	markerSequenceManager.sequenceStore.Store(NewSequence(1, Markers{
		&Marker{0, 1},
	}, 1))
	markerSequenceManager.sequenceStore.Store(NewSequence(2, Markers{
		&Marker{0, 7},
	}, 1))

	markerSequenceManager.NormalizeMarkers(Markers{
		&Marker{0, 3},
		&Marker{1, 7},
		&Marker{2, 9},
	})

	fmt.Println(markerSequenceManager)
}
