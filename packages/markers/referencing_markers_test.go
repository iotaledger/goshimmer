package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
