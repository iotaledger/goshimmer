package marker

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

type Index uint64

func IndexFromBytes(sequenceBytes []byte) (index Index, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if index, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Index from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func IndexFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (index Index, err error) {
	untypedIndex, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Index (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	index = Index(untypedIndex)

	return
}

func (i Index) Bytes() []byte {
	return marshalutil.New(marshalutil.UINT64_SIZE).
		WriteUint64(uint64(i)).
		Bytes()
}

func (i Index) String() string {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Marker struct {
	sequenceID SequenceID
	index      Index
}

func New(sequenceID SequenceID, index Index) *Marker {
	return &Marker{
		sequenceID: sequenceID,
		index:      index,
	}
}

func FromBytes(markerBytes []byte) (marker *Marker, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markerBytes)
	if marker, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	return
}

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

func (m *Marker) SequenceID() SequenceID {
	return m.sequenceID
}

func (m *Marker) Index() Index {
	return m.index
}

func (m *Marker) Bytes() []byte {
	return marshalutil.New(marshalutil.UINT64_SIZE + marshalutil.UINT64_SIZE).
		Write(m.sequenceID).
		Write(m.index).
		Bytes()
}

func (m *Marker) String() string {
	return stringify.Struct("Marker",
		stringify.StructField("sequenceID", m.SequenceID()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

type Markers map[SequenceID]Index

func MarkersFromBytes(markersBytes []byte) (markers Markers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markersBytes)
	if markers, err = MarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func MarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markers Markers, err error) {
	markersCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse Markers count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	markers = make(Markers)
	for i := 0; i < int(markersCount); i++ {
		sequenceID, sequenceIDErr := SequenceIDFromMarshalUtil(marshalUtil)
		if sequenceIDErr != nil {
			err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", sequenceIDErr)
			return
		}
		index, indexErr := IndexFromMarshalUtil(marshalUtil)
		if indexErr != nil {
			err = xerrors.Errorf("failed to parse Index from MarshalUtil: %w", indexErr)
			return
		}
		markers[sequenceID] = index
	}

	return
}

func (m Markers) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(m)))
	for sequenceID, index := range m {
		marshalUtil.Write(sequenceID).Write(index)
	}
	return marshalUtil.Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
