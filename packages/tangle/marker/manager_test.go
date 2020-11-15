package marker

import (
	"fmt"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
)

func TestMarkersManager_InheritMarkers(t *testing.T) {
	markerSequenceManager := NewManager(mapdb.NewMapDB())

	inheritedMarkers, isNewMarker := markerSequenceManager.InheritMarkers(NewMarkers())

	fmt.Println(inheritedMarkers, isNewMarker)
	fmt.Println(markerSequenceManager.InheritMarkers(NewMarkers(), NewSequenceAlias([]byte("testBranch"))))
	fmt.Println(markerSequenceManager.InheritMarkers(inheritedMarkers))
}

func Test(t *testing.T) {
	manager := NewManager(mapdb.NewMapDB())

	manager.sequenceStore.Store(NewSequence(0, NewMarkers(), 0))
	manager.sequenceStore.Store(NewSequence(1, NewMarkers(
		&Marker{0, 1},
	), 1))
	manager.sequenceStore.Store(NewSequence(2, NewMarkers(
		&Marker{0, 7},
	), 1))
	manager.sequenceStore.Store(NewSequence(3, NewMarkers(
		&Marker{1, 6},
		&Marker{2, 1},
	), 2))

	normalizedMarkers, normalizedSequences, rank := manager.NormalizeMarkers(
		manager.MergeMarkers(
			NewMarkers(
				&Marker{1, 7},
			), NewMarkers(
				&Marker{0, 3},
				&Marker{2, 9},
			), NewMarkers(
				&Marker{3, 7},
			),
		),
	)

	fmt.Println(normalizedMarkers, normalizedSequences, rank)
}
