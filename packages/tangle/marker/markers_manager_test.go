package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test(t *testing.T) {
	markerSequenceManager := NewMarkersManager(mapdb.NewMapDB())

	markerSequenceManager.sequenceStore.Store(NewSequence(0, NewNormalizedMarkers(), 0))
	markerSequenceManager.sequenceStore.Store(NewSequence(1, NewNormalizedMarkers(
		&Marker{0, 1},
	), 1))
	markerSequenceManager.sequenceStore.Store(NewSequence(2, NewNormalizedMarkers(
		&Marker{0, 7},
	), 1))

	normalizedMarkers, rank := markerSequenceManager.NormalizeMarkers(NewNormalizedMarkers(
		&Marker{0, 3},
		&Marker{1, 7},
		&Marker{2, 9},
	))

	fmt.Println(normalizedMarkers, rank)
}
