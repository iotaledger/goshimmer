package markers

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerixReferencedMarkers(t *testing.T) {
	obj := NewReferencedMarkers(NewMarkers(NewMarker(1, 5)))

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	restoredObj, _, err := ReferencedMarkersFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())

}

func TestSerixReferencingMarkers(t *testing.T) {
	obj := NewReferencingMarkers()
	obj.Add(3, NewMarker(1, 5))
	obj.Add(5, NewMarker(2, 5))

	//assert.Equal(t, obj.BytesOld(), obj.Bytes())

	restoredObj, _, err := ReferencingMarkersFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())
}

func TestSerixSequence(t *testing.T) {
	obj := NewSequence(1, NewMarkers(NewMarker(1, 5)))
	obj.AddReferencingMarker(3, NewMarker(1, 5))
	obj.AddReferencingMarker(5, NewMarker(2, 5))
	assert.Equal(t, obj.ObjectStorageKeyOld(), obj.ObjectStorageKey())
	assert.Equal(t, obj.ObjectStorageValueOld(), obj.ObjectStorageValue())

	restoredObj, err := new(Sequence).FromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())

	restoredObj2, err := new(Sequence).FromObjectStorage(obj.ObjectStorageKey(), obj.ObjectStorageValue())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj2.(*Sequence).Bytes())
}

func TestSerixMarker(t *testing.T) {
	obj := NewMarker(1, 2)

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	restoredObj, _, err := MarkerFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())
}

func TestSerixMarkers(t *testing.T) {
	obj := NewMarkers(NewMarker(1, 2))

	assert.Equal(t, obj.BytesOld(), obj.Bytes())

	restoredObj, _, err := FromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())
}

func TestSerixStructureDetails(t *testing.T) {
	obj := &StructureDetails{
		Rank:          0,
		PastMarkerGap: 10,
		IsPastMarker:  false,
		PastMarkers:   NewMarkers(NewMarker(1, 2)),
		FutureMarkers: NewMarkers(NewMarker(1, 5)),
	}

	assert.Equal(t, obj.BytesOld()[4:], obj.Bytes())
	//TODO: replace with FromBytes
	restoredObj, _, err := StructureDetailsFromBytes(obj.Bytes())
	assert.NoError(t, err)
	assert.Equal(t, obj.Bytes(), restoredObj.Bytes())
}
