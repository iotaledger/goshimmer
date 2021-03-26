package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
