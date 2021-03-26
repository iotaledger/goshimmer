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
	), childReferences.ReferencingMarkers(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), childReferences.ReferencingMarkers(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), childReferences.ReferencingMarkers(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), childReferences.ReferencingMarkers(12))

	assert.Equal(t, NewMarkers(), childReferences.ReferencingMarkers(13))

	marshaledChildReferences := childReferences.Bytes()
	unmarshaledChildReferences, consumedBytes, err := ChildReferencesFromBytes(marshaledChildReferences)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledChildReferences), consumedBytes)

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.ReferencingMarkers(8))

	assert.Equal(t, NewMarkers(
		&Marker{1, 5},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.ReferencingMarkers(9))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
		&Marker{3, 4},
	), unmarshaledChildReferences.ReferencingMarkers(10))

	assert.Equal(t, NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), unmarshaledChildReferences.ReferencingMarkers(12))

	assert.Equal(t, NewMarkers(), unmarshaledChildReferences.ReferencingMarkers(13))

	fmt.Println(unmarshaledChildReferences)
}
