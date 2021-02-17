package markers

import (
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	Rank                     uint64
	IsPastMarker             bool
	PastMarkers              *Markers
	FutureMarkers            *Markers
	futureMarkersUpdateMutex sync.Mutex
}

// StructureDetailsFromBytes unmarshals a StructureDetails object from a sequence of bytes.
func StructureDetailsFromBytes(markersBytes []byte) (markersPair *StructureDetails, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markersBytes)
	if markersPair, err = StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// StructureDetailsFromMarshalUtil unmarshals a StructureDetails using a MarshalUtil (for easier unmarshaling).
func StructureDetailsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markersPair *StructureDetails, err error) {
	detailsExist, err := marshalUtil.ReadBool()
	if err != nil {
		err = xerrors.Errorf("failed to parse exists flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if !detailsExist {
		return
	}

	markersPair = &StructureDetails{}
	if markersPair.Rank, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse Rank (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if markersPair.IsPastMarker, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse IsPastMarker (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if markersPair.PastMarkers, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse PastMarkers from MarshalUtil: %w", err)
		return
	}
	if markersPair.FutureMarkers, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse FutureMarkers from MarshalUtil: %w", err)
		return
	}

	return
}

// Clone creates a deep copy of the StructureDetails.
func (m *StructureDetails) Clone() (clone *StructureDetails) {
	return &StructureDetails{
		Rank:          m.Rank,
		IsPastMarker:  m.IsPastMarker,
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
		WriteBool(m.IsPastMarker).
		Write(m.PastMarkers).
		Write(m.FutureMarkers).
		Bytes()
}

// String returns a human readable version of the StructureDetails.
func (m *StructureDetails) String() (humanReadableStructureDetails string) {
	return stringify.Struct("StructureDetails",
		stringify.StructField("Rank", m.Rank),
		stringify.StructField("IsPastMarker", m.IsPastMarker),
		stringify.StructField("PastMarkers", m.PastMarkers),
		stringify.StructField("FutureMarkers", m.FutureMarkers),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
