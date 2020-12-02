package markers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSequence(t *testing.T) {
	sequence := NewSequence(1337, NewMarkers(
		&Marker{1, 3},
		&Marker{2, 6},
	), 7)

	assert.Equal(t, SequenceID(1337), sequence.ID())
	assert.Equal(t, NewSequenceIDs(1, 2), sequence.ParentSequences())
	assert.Equal(t, uint64(7), sequence.Rank())
	assert.Equal(t, Index(7), sequence.HighestIndex())

	marshaledSequence := sequence.Bytes()
	unmarshaledSequence, consumedBytes, err := SequenceFromBytes(marshaledSequence)
	require.NoError(t, err)
	assert.Equal(t, len(marshaledSequence), consumedBytes)
	assert.Equal(t, sequence.ID(), unmarshaledSequence.ID())
	assert.Equal(t, sequence.Rank(), unmarshaledSequence.Rank())
	assert.Equal(t, sequence.HighestIndex(), unmarshaledSequence.HighestIndex())
	assert.Equal(t, sequence.ParentSequences(), unmarshaledSequence.ParentSequences())
}
