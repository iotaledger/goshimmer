package marker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequence(t *testing.T) {
	sequence := NewSequence(1337, NewNormalizedMarkers(
		&Marker{1, 3},
		&Marker{2, 6},
	), 7)

	assert.Equal(t, SequenceID(1337), sequence.ID())
	assert.Equal(t, NewSequenceIDs(1, 2), sequence.ParentSequences())
	assert.Equal(t, uint64(7), sequence.Rank())
	assert.Equal(t, Index(1338), sequence.HighestIndex())

	marshaledSequence := sequence.Bytes()
	unmarshaledSequence, consumedBytes, err := SequenceFromBytes(marshaledSequence)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledSequence), consumedBytes)
	assert.Equal(t, sequence, unmarshaledSequence)
}
