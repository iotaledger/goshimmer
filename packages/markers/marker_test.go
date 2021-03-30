package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarker(t *testing.T) {
	marker := &Marker{SequenceID(1337), Index(1)}
	assert.Equal(t, SequenceID(1337), marker.SequenceID())
	assert.Equal(t, Index(1), marker.Index())

	marshaledMarker := marker.Bytes()
	unmarshaledMarker, consumedBytes, err := MarkerFromBytes(marshaledMarker)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarker), consumedBytes)
	assert.Equal(t, marker, unmarshaledMarker)
}

func TestMarkers(t *testing.T) {
	markers := NewMarkers(
		&Marker{1337, 1},
		&Marker{1338, 2},
		&Marker{1339, 3},
	)

	marshaledMarkers := markers.Bytes()
	unmarshaledMarkers, consumedBytes, err := FromBytes(marshaledMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarkers), consumedBytes)
	assert.Equal(t, markers, unmarshaledMarkers)
	assert.Equal(t, Index(3), markers.HighestIndex())
	assert.Equal(t, Index(1), markers.LowestIndex())

	markers.Delete(1337)
	assert.Equal(t, Index(2), markers.LowestIndex())
	assert.Equal(t, Index(3), markers.HighestIndex())

	markers.Delete(1339)
	assert.Equal(t, Index(2), markers.LowestIndex())
	assert.Equal(t, Index(2), markers.HighestIndex())
}

func TestMarkersByRank(t *testing.T) {
	markersByRank := newMarkersByRank()

	updated, added := markersByRank.Add(10, 7, 8)
	assert.True(t, updated)
	assert.True(t, added)
	updated, added = markersByRank.Add(10, 7, 8)
	assert.False(t, updated)
	assert.False(t, added)
	updated, added = markersByRank.Add(10, 7, 9)
	assert.True(t, updated)
	assert.False(t, added)
	assert.Equal(t, uint64(10), markersByRank.LowestRank())
	assert.Equal(t, uint64(10), markersByRank.HighestRank())
	assert.Equal(t, uint64(1), markersByRank.Size())

	updated, added = markersByRank.Add(12, 7, 9)
	assert.True(t, updated)
	assert.True(t, added)
	assert.Equal(t, uint64(10), markersByRank.LowestRank())
	assert.Equal(t, uint64(12), markersByRank.HighestRank())
	assert.Equal(t, uint64(2), markersByRank.Size())

	assert.False(t, markersByRank.Delete(10, 8))
	assert.True(t, markersByRank.Delete(10, 7))
	assert.Equal(t, uint64(12), markersByRank.LowestRank())
	assert.Equal(t, uint64(12), markersByRank.HighestRank())
	assert.Equal(t, uint64(1), markersByRank.Size())

	assert.True(t, markersByRank.Delete(12, 7))
	assert.Equal(t, uint64(1<<64-1), markersByRank.LowestRank())
	assert.Equal(t, uint64(0), markersByRank.HighestRank())
	assert.Equal(t, uint64(0), markersByRank.Size())
}

func TestReferencedMarkers(t *testing.T) {
	referencedMarkers := NewReferencedMarkers(NewMarkers(
		&Marker{1, 3},
		&Marker{2, 7},
	))

	referencedMarkers.Add(8, NewMarkers(
		&Marker{4, 9},
	))

	referencedMarkers.Add(9, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
	))

	referencedMarkers.Add(12, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	))

	assert.Equal(t, NewMarkers(
		&Marker{1, 3},
		&Marker{2, 7},
		&Marker{4, 9},
	), referencedMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
		&Marker{4, 9},
	), referencedMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
		&Marker{4, 9},
	), referencedMarkers.Get(11))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{4, 9},
	), referencedMarkers.Get(12))

	marshaledReferencedMarkers := referencedMarkers.Bytes()
	unmarshaledReferencedMarkers, consumedBytes, err := ReferencedMarkersFromBytes(marshaledReferencedMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledReferencedMarkers), consumedBytes)

	assert.Equal(t, NewMarkers(
		&Marker{1, 3},
		&Marker{2, 7},
		&Marker{4, 9},
	), unmarshaledReferencedMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
		&Marker{4, 9},
	), unmarshaledReferencedMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
		&Marker{4, 9},
	), unmarshaledReferencedMarkers.Get(11))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{4, 9},
	), unmarshaledReferencedMarkers.Get(12))

	fmt.Println(unmarshaledReferencedMarkers)
}

func TestReferencedMarkersPanic(t *testing.T) {
	referencedMarkers := NewReferencedMarkers(NewMarkers(
		&Marker{1, 3},
	))

	referencedMarkers.Add(7, NewMarkers(
		&Marker{4, 9},
	))

	assert.Equal(t, NewMarkers(
		&Marker{1, 3},
	), referencedMarkers.Get(4))
}

func TestReferencingMarkers(t *testing.T) {
	referencingMarkers := NewReferencingMarkers()
	referencingMarkers.Add(9, &Marker{1, 5})
	referencingMarkers.Add(10, &Marker{3, 4})
	referencingMarkers.Add(12, &Marker{1, 7})
	referencingMarkers.Add(12, &Marker{2, 10})

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), referencingMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), referencingMarkers.Get(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), referencingMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), referencingMarkers.Get(12))

	assert.Equal(t, NewMarkers(), referencingMarkers.Get(13))

	marshaledReferencingMarkers := referencingMarkers.Bytes()
	unmarshaledReferencingMarkers, consumedBytes, err := ReferencingMarkersFromBytes(marshaledReferencingMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledReferencingMarkers), consumedBytes)

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledReferencingMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledReferencingMarkers.Get(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledReferencingMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), unmarshaledReferencingMarkers.Get(12))

	assert.Equal(t, NewMarkers(), unmarshaledReferencingMarkers.Get(13))

	fmt.Println(unmarshaledReferencingMarkers)
}
