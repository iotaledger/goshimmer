package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMarker(t *testing.T) {
	marker := New(SequenceID(1337), Index(1))
	assert.Equal(t, SequenceID(1337), marker.SequenceID())
	assert.Equal(t, Index(1), marker.Index())

	marshaledMarker := marker.Bytes()
	unmarshaledMarker, consumedBytes, err := FromBytes(marshaledMarker)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarker), consumedBytes)
	assert.Equal(t, marker, unmarshaledMarker)
}

func TestMarkers(t *testing.T) {
	markers := Markers{
		&Marker{1337, 1},
		&Marker{1338, 2},
		&Marker{1339, 3},
	}

	marshaledMarkers := markers.Bytes()
	unmarshaledMarkers, consumedBytes, err := MarkersFromBytes(marshaledMarkers)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledMarkers), consumedBytes)
	assert.Equal(t, markers, unmarshaledMarkers)
}
