package markers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParentReferences(t *testing.T) {
	parentReferences := NewParentReferences(NewMarkers(
		&Marker{1, 3},
		&Marker{2, 7},
	))
	assert.Equal(t, NewSequenceIDs(1, 2), parentReferences.ParentSequences())

	parentReferences.AddReferences(NewMarkers(
		&Marker{1, 5},
		&Marker{2, 8},
	), 9)

	parentReferences.AddReferences(NewMarkers(
		&Marker{1, 7},
		&Marker{2, 10},
	), 12)

	assert.Equal(t, NewSequenceIDs(1, 2), parentReferences.SequenceIDs())
	assert.Equal(t, &Marker{1, 3}, parentReferences.HighestReferencedMarker(1, 8))
	assert.Equal(t, &Marker{1, 5}, parentReferences.HighestReferencedMarker(1, 10))
	assert.Equal(t, &Marker{1, 5}, parentReferences.HighestReferencedMarker(1, 11))
	assert.Equal(t, &Marker{1, 7}, parentReferences.HighestReferencedMarker(1, 12))

	marshaledParentReferences := parentReferences.Bytes()
	unmarshalParentReferences, consumedBytes, err := ParentReferencesFromBytes(marshaledParentReferences)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledParentReferences), consumedBytes)

	assert.Equal(t, NewSequenceIDs(1, 2), unmarshalParentReferences.SequenceIDs())
	assert.Equal(t, &Marker{1, 3}, unmarshalParentReferences.HighestReferencedMarker(1, 8))
	assert.Equal(t, &Marker{1, 5}, unmarshalParentReferences.HighestReferencedMarker(1, 10))
	assert.Equal(t, &Marker{1, 5}, unmarshalParentReferences.HighestReferencedMarker(1, 11))
	assert.Equal(t, &Marker{1, 7}, unmarshalParentReferences.HighestReferencedMarker(1, 12))

	fmt.Println(unmarshalParentReferences)
}
