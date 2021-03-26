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

	referencedMarkers.AddReferences(NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
	), 9)

	referencedMarkers.AddReferences(NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), 12)

	assert.Equal(t, &Marker{1, 3}, referencedMarkers.HighestReferencedMarker(1, 8))
	assert.Equal(t, &Marker{1, 5}, referencedMarkers.HighestReferencedMarker(1, 10))
	assert.Equal(t, &Marker{1, 5}, referencedMarkers.HighestReferencedMarker(1, 11))
	assert.Equal(t, &Marker{1, 7}, referencedMarkers.HighestReferencedMarker(1, 12))

	marshaledReferencedMarkers := referencedMarkers.Bytes()
	unmarshaledReferencedMarkers, consumedBytes, err := ReferencedMarkersFromBytes(marshaledReferencedMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledReferencedMarkers), consumedBytes)

	assert.Equal(t, &Marker{1, 3}, unmarshaledReferencedMarkers.HighestReferencedMarker(1, 8))
	assert.Equal(t, &Marker{1, 5}, unmarshaledReferencedMarkers.HighestReferencedMarker(1, 10))
	assert.Equal(t, &Marker{1, 5}, unmarshaledReferencedMarkers.HighestReferencedMarker(1, 11))
	assert.Equal(t, &Marker{1, 7}, unmarshaledReferencedMarkers.HighestReferencedMarker(1, 12))

	fmt.Println(unmarshaledReferencedMarkers)
}
