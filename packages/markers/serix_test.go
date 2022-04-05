package markers

import (
	"context"
	"testing"

	"github.com/iotaledger/hive.go/serix"
	"github.com/stretchr/testify/assert"
)

func TestSerixReferencedMarkers(t *testing.T) {
	obj := NewReferencedMarkers(NewMarkers(NewMarker(1, 5)))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixReferencingMarkers(t *testing.T) {
	obj := NewReferencingMarkers()
	obj.Add(3, NewMarker(1, 5))
	obj.Add(5, NewMarker(2, 5))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixSequence(t *testing.T) {
	obj := NewSequence(1, NewMarkers(NewMarker(1, 5)))
	obj.AddReferencingMarker(3, NewMarker(1, 5))
	obj.AddReferencingMarker(5, NewMarker(2, 5))
	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)
	assert.Equal(t, obj.ObjectStorageValue(), serixBytes)
}

func TestSerixMarker(t *testing.T) {
	obj := NewMarker(1, 2)

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.Bytes(), serixBytes)
}

func TestSerixMarkers(t *testing.T) {
	obj := NewMarkers(NewMarker(1, 2))

	serixBytes, err := serix.DefaultAPI.Encode(context.Background(), obj)
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
	serixBytesKey, err := serix.DefaultAPI.Encode(context.Background(), obj)
	assert.NoError(t, err)

	assert.Equal(t, obj.Bytes(), serixBytesKey)
}
