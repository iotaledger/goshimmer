package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChildReferences(t *testing.T) {
	childReferences := NewChildReferences()
	childReferences.AddReferencingMarker(9, &Marker{1, 5})
	childReferences.AddReferencingMarker(10, &Marker{3, 4})
	childReferences.AddReferencingMarker(12, &Marker{1, 7})
	childReferences.AddReferencingMarker(12, &Marker{2, 10})

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), childReferences.LowestReferencingMarkers(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), childReferences.LowestReferencingMarkers(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), childReferences.LowestReferencingMarkers(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), childReferences.LowestReferencingMarkers(12))

	assert.Equal(t, NewMarkers(), childReferences.LowestReferencingMarkers(13))

	marshaledChildReferences := childReferences.Bytes()
	unmarshaledChildReferences, consumedBytes, err := ChildReferencesFromBytes(marshaledChildReferences)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledChildReferences), consumedBytes)

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.LowestReferencingMarkers(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.LowestReferencingMarkers(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.LowestReferencingMarkers(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), unmarshaledChildReferences.LowestReferencingMarkers(12))

	assert.Equal(t, NewMarkers(), unmarshaledChildReferences.LowestReferencingMarkers(13))

	fmt.Println(unmarshaledChildReferences)
}
