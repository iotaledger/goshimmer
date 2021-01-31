package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessageMetadataFromBytes(t *testing.T) {
	m := NewMessageMetadata(EmptyMessageID)
	details := &markers.StructureDetails{
		Rank:          22,
		IsPastMarker:  true,
		PastMarkers:   markers.NewMarkers(),
		FutureMarkers: markers.NewMarkers(),
	}
	m.SetStructureDetails(details)
	bytes := m.Bytes()

	m2, consumedBytes, err := MessageMetadataFromBytes(bytes)
	require.NoError(t, err)
	assert.Equal(t, len(bytes), consumedBytes)
	assert.Equal(t, bytes, m2.Bytes())
}
