package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarker(t *testing.T) {
	marker := NewMarker(SequenceID(1337), Index(1))
	assert.Equal(t, SequenceID(1337), marker.SequenceID())
	assert.Equal(t, Index(1), marker.Index())

	marshaledMarker := marker.Bytes()
	unmarshalledMarker, consumedBytes, err := MarkerFromBytes(marshaledMarker)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarker), consumedBytes)
	assert.Equal(t, marker, unmarshalledMarker)
}

func TestMarkers(t *testing.T) {
	markers := NewMarkers(
		NewMarker(1337, 1),
		NewMarker(1338, 2),
		NewMarker(1339, 3),
	)

	marshaledMarkers := markers.Bytes()
	unmarshalledMarkers, consumedBytes, err := FromBytes(marshaledMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarkers), consumedBytes)
	assert.Equal(t, markers, unmarshalledMarkers)
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
		NewMarker(1, 3),
		NewMarker(2, 7),
	))

	referencedMarkers.Add(8, NewMarkers(
		NewMarker(4, 9),
	))

	referencedMarkers.Add(9, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
	))

	referencedMarkers.Add(12, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
	))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 3),
		NewMarker(2, 7),
		NewMarker(4, 9),
	), referencedMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), referencedMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), referencedMarkers.Get(11))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(4, 9),
	), referencedMarkers.Get(12))

	marshaledReferencedMarkers := referencedMarkers.Bytes()
	unmarshalledReferencedMarkers, consumedBytes, err := ReferencedMarkersFromBytes(marshaledReferencedMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledReferencedMarkers), consumedBytes)

	assert.Equal(t, NewMarkers(
		NewMarker(1, 3),
		NewMarker(2, 7),
		NewMarker(4, 9),
	), unmarshalledReferencedMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), unmarshalledReferencedMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), unmarshalledReferencedMarkers.Get(11))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(4, 9),
	), unmarshalledReferencedMarkers.Get(12))
}

func TestReferencedMarkersPanic(t *testing.T) {
	referencedMarkers := NewReferencedMarkers(NewMarkers(
		NewMarker(1, 3),
	))

	referencedMarkers.Add(7, NewMarkers(
		NewMarker(4, 9),
	))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 3),
	), referencedMarkers.Get(4))
}

func TestReferencingMarkers(t *testing.T) {
	referencingMarkers := NewReferencingMarkers()
	referencingMarkers.Add(9, NewMarker(1, 5))
	referencingMarkers.Add(10, NewMarker(3, 4))
	referencingMarkers.Add(12, NewMarker(1, 7))
	referencingMarkers.Add(12, NewMarker(2, 10))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(9))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
	), referencingMarkers.Get(12))

	assert.Equal(t, NewMarkers(), referencingMarkers.Get(13))

	marshaledReferencingMarkers := referencingMarkers.Bytes()
	unmarshalledReferencingMarkers, consumedBytes, err := ReferencingMarkersFromBytes(marshaledReferencingMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledReferencingMarkers), consumedBytes)

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), unmarshalledReferencingMarkers.Get(8))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), unmarshalledReferencingMarkers.Get(9))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), unmarshalledReferencingMarkers.Get(10))

	assert.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
	), unmarshalledReferencingMarkers.Get(12))

	assert.Equal(t, NewMarkers(), unmarshalledReferencingMarkers.Get(13))

	fmt.Println(unmarshalledReferencingMarkers)
}

func TestSequence(t *testing.T) {
	sequence := NewSequence(1337, NewMarkers(
		NewMarker(1, 3),
		NewMarker(2, 6),
	))

	assert.Equal(t, SequenceID(1337), sequence.ID())
	assert.Equal(t, Index(7), sequence.HighestIndex())

	marshaledSequence := sequence.Bytes()
	unmarshalledSequence, err := new(Sequence).FromBytes(marshaledSequence)
	require.NoError(t, err)
	assert.Equal(t, sequence.ID(), unmarshalledSequence.ID())
	assert.Equal(t, sequence.HighestIndex(), unmarshalledSequence.HighestIndex())
}
