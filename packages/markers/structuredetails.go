package markers

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	Rank                     uint64
	PastMarkerGap            uint64
	IsPastMarker             bool
	SequenceID               SequenceID
	PastMarkers              *Markers
	FutureMarkers            *Markers
	futureMarkersUpdateMutex sync.Mutex
}

// StructureDetailsFromBytes unmarshals a StructureDetails object from a sequence of bytes.
func StructureDetailsFromBytes(markersBytes []byte) (markersPair *StructureDetails, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markersBytes)
	if markersPair, err = StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// StructureDetailsFromMarshalUtil unmarshals a StructureDetails using a MarshalUtil (for easier unmarshaling).
func StructureDetailsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (structureDetails *StructureDetails, err error) {
	detailsExist, err := marshalUtil.ReadBool()
	if err != nil {
		err = errors.Errorf("failed to parse exists flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if !detailsExist {
		return
	}

	structureDetails = &StructureDetails{}
	if structureDetails.Rank, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse Rank (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if structureDetails.PastMarkerGap, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse PastMarkerGap (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if structureDetails.IsPastMarker, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse IsPastMarker (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if structureDetails.SequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	if structureDetails.PastMarkers, err = FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse PastMarkers from MarshalUtil: %w", err)
		return
	}
	if structureDetails.FutureMarkers, err = FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse FutureMarkers from MarshalUtil: %w", err)
		return
	}

	return
}

// Clone creates a deep copy of the StructureDetails.
func (m *StructureDetails) Clone() (clone *StructureDetails) {
	return &StructureDetails{
		Rank:          m.Rank,
		PastMarkerGap: m.PastMarkerGap,
		IsPastMarker:  m.IsPastMarker,
		SequenceID:    m.SequenceID,
		PastMarkers:   m.PastMarkers.Clone(),
		FutureMarkers: m.FutureMarkers.Clone(),
	}
}

// Bytes returns a marshaled version of the StructureDetails.
func (m *StructureDetails) Bytes() (marshaledStructureDetails []byte) {
	if m == nil {
		return marshalutil.New(marshalutil.BoolSize).WriteBool(false).Bytes()
	}

	return marshalutil.New().
		WriteBool(true).
		WriteUint64(m.Rank).
		WriteUint64(m.PastMarkerGap).
		WriteBool(m.IsPastMarker).
		Write(m.SequenceID).
		Write(m.PastMarkers).
		Write(m.FutureMarkers).
		Bytes()
}

// String returns a human readable version of the StructureDetails.
func (m *StructureDetails) String() (humanReadableStructureDetails string) {
	return stringify.Struct("StructureDetails",
		stringify.StructField("Rank", m.Rank),
		stringify.StructField("PastMarkerGap", m.PastMarkerGap),
		stringify.StructField("IsPastMarker", m.IsPastMarker),
		stringify.StructField("SequenceID", m.SequenceID),
		stringify.StructField("PastMarkers", m.PastMarkers),
		stringify.StructField("FutureMarkers", m.FutureMarkers),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
