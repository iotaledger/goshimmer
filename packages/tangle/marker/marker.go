package marker

import (
	"strconv"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Index identifies a marker in a marker sequence.
type Index uint64

// IndexFromBytes unmarshals a marker index from a sequence of bytes.
func IndexFromBytes(sequenceBytes []byte) (index Index, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if index, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Index from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IndexFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func IndexFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (index Index, err error) {
	untypedIndex, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Index (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	index = Index(untypedIndex)

	return
}

// Bytes returns the bytes of the Index.
func (i Index) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint64Size).
		WriteUint64(uint64(i)).
		Bytes()
}

// String returns the base58 encode of the Index.
func (i Index) String() string {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Marker represents a selected message from the Tangle and forms a marker sequence.
type Marker struct {
	sequenceID SequenceID
	index      Index
}

// New creates a new marker.
func New(sequenceID SequenceID, index Index) *Marker {
	return &Marker{
		sequenceID: sequenceID,
		index:      index,
	}
}

// FromBytes unmarshals a marker from a sequence of bytes.
func FromBytes(markerBytes []byte) (marker *Marker, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markerBytes)
	if marker, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// FromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (marker *Marker, err error) {
	marker = &Marker{}
	if marker.sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	if marker.index, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Index from MarshalUtil: %w", err)
		return
	}

	return
}

// SequenceID returns the id of the marker sequence of the marker.
func (m *Marker) SequenceID() SequenceID {
	return m.sequenceID
}

// Index returns the index of the marker in a marker sequence.
func (m *Marker) Index() Index {
	return m.index
}

// Bytes returns the marker in serialized byte form.
func (m *Marker) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint64Size + marshalutil.Uint64Size).
		Write(m.sequenceID).
		Write(m.index).
		Bytes()
}

// String returns the base58 encode of the Marker.
func (m *Marker) String() string {
	return stringify.Struct("Marker",
		stringify.StructField("sequenceID", m.SequenceID()),
		stringify.StructField("index", m.Index()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a list of Marker.
type Markers []*Marker

// MarkersFromBytes unmarshals a collection of Markers from a sequence of bytes.
func MarkersFromBytes(markersBytes []byte) (markers Markers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markersBytes)
	if markers, err = MarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MarkersFromMarshalUtil unmarshals a collection of Markers using a MarshalUtil (for easier unmarshaling).
func MarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markers Markers, err error) {
	markersCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse Markers count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	markers = make(Markers, markersCount)
	for i := 0; i < int(markersCount); i++ {
		if markers[i], err = FromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
			return
		}
	}

	return
}

// HighestIndex returns the highest index of Markers
func (m Markers) HighestIndex() (highestMarker Index) {
	for _, marker := range m {
		if marker.index > highestMarker {
			highestMarker = marker.index
		}
	}

	return
}

// Bytes returns the Markers in serialized byte form.
func (m Markers) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(m)))
	for _, marker := range m {
		marshalUtil.Write(marker)
	}

	return marshalUtil.Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// UniqueMarkers represents a list markers from different marker sequences.
type UniqueMarkers map[SequenceID]Index
