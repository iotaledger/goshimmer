package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func Test(t *testing.T) {
	markerSequenceManager := NewMarkersManager(mapdb.NewMapDB())

	markerSequenceManager.sequenceStore.Store(NewSequence(0, NewMarkers(), 0))
	markerSequenceManager.sequenceStore.Store(NewSequence(1, NewMarkers(
		&Marker{0, 1},
	), 1))
	markerSequenceManager.sequenceStore.Store(NewSequence(2, NewMarkers(
		&Marker{0, 7},
	), 1))

	normalizedMarkers, rank := markerSequenceManager.NormalizeMarkers(NewMarkers(
		&Marker{1, 7},
	), NewMarkers(
		&Marker{0, 3},
		&Marker{2, 9},
	))

	fmt.Println(normalizedMarkers, rank)
}
