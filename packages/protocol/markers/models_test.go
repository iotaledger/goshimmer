package markers

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

	require.Equal(t, NewMarkers(
		NewMarker(1, 3),
		NewMarker(2, 7),
		NewMarker(4, 9),
	), referencedMarkers.Get(8))

	require.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), referencedMarkers.Get(10))

	require.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 8),
		NewMarker(4, 9),
	), referencedMarkers.Get(11))

	require.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(4, 9),
	), referencedMarkers.Get(12))
}

func TestReferencedMarkersPanic(t *testing.T) {
	referencedMarkers := NewReferencedMarkers(NewMarkers(
		NewMarker(1, 3),
	))

	referencedMarkers.Add(7, NewMarkers(
		NewMarker(4, 9),
	))

	require.Equal(t, NewMarkers(
		NewMarker(1, 3),
	), referencedMarkers.Get(4))
}

func TestReferencingMarkers(t *testing.T) {
	referencingMarkers := NewReferencingMarkers()
	referencingMarkers.Add(9, NewMarker(1, 5))
	referencingMarkers.Add(10, NewMarker(3, 4))
	referencingMarkers.Add(12, NewMarker(1, 7))
	referencingMarkers.Add(12, NewMarker(2, 10))

	require.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(8))

	require.Equal(t, NewMarkers(
		NewMarker(1, 5),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(9))

	require.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
		NewMarker(3, 4),
	), referencingMarkers.Get(10))

	require.Equal(t, NewMarkers(
		NewMarker(1, 7),
		NewMarker(2, 10),
	), referencingMarkers.Get(12))

	require.Equal(t, NewMarkers(), referencingMarkers.Get(13))
}

func TestSequence(t *testing.T) {
	sequence := NewSequence(1337, NewMarkers(
		NewMarker(1, 3),
		NewMarker(2, 6),
	))

	require.Equal(t, SequenceID(1337), sequence.ID())
	require.Equal(t, Index(7), sequence.HighestIndex())
}
