package markers

import (
	"context"
	"testing"

	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"
)

func TestSerixSequence(t *testing.T) {

	// TODO: thresholdmap
	obj := NewSequence(1, NewMarkers(NewMarker(1, 5)))

	s := serix.NewAPI()

	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)
	// Skip OutputID and TransactionID which are serialized by the Bytes method, but are used only as a object storage key.
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixMarker(t *testing.T) {
	obj := NewMarker(1, 2)

	s := serix.NewAPI()
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixMarkers(t *testing.T) {
	obj := NewMarkers(NewMarker(1, 2), NewMarker(2, 3))

	s := serix.NewAPI()
	serixBytes, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixStructureDetails(t *testing.T) {
	obj := &StructureDetails{
		Rank:          0,
		PastMarkerGap: 10,
		IsPastMarker:  false,
		PastMarkers:   NewMarkers(NewMarker(1, 2)),
		FutureMarkers: NewMarkers(NewMarker(1, 5)),
	}
	s := serix.NewAPI()
	serixBytesKey, err := s.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.Bytes(), serixBytesKey)
}
