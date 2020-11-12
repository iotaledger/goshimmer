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

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers map[SequenceID]Index

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

func NewMarkers(optionalMarkers ...*Marker) (markers Markers) {
	markers = make(Markers)
	for _, marker := range optionalMarkers {
		markers.Set(marker.sequenceID, marker.index)
	}

	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and added.
func (m Markers) Set(sequenceID SequenceID, index Index) (updated bool, added bool) {
	if existingIndex, indexAlreadyStored := m[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m[sequenceID] = index
		}
		return
	}

	m[sequenceID] = index
	updated = true
	added = true

	return
}

func (m Markers) HighestIndex() (highestMarker Index) {
	for _, index := range m {
		if index > highestMarker {
			highestMarker = index
		}
	}

	return
}

// SequenceIDs returns the SequenceIDs that the normalized Markers represent.
func (m Markers) SequenceIDs() SequenceIDs {
	sequenceIDs := make([]SequenceID, 0, len(m))
	for sequenceID := range m {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}

	return NewSequenceIDs(sequenceIDs...)
}

// Clone create a copy of the Markers.
func (m Markers) Clone() (clone Markers) {
	clone = make(Markers)
	for sequenceID, index := range m {
		clone[sequenceID] = index
	}

	return
}

// Bytes returns the Markers in serialized byte form.
func (m Markers) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(m)))
	for sequenceID, index := range m {
		marshalUtil.Write(sequenceID)
		marshalUtil.Write(index)
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Markers.
func (m Markers) String() string {
	structBuilder := stringify.StructBuilder("Markers")
	for sequenceID, index := range m {
		structBuilder.AddField(stringify.StructField(sequenceID.String(), index))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MarkersByRank ////////////////////////////////////////////////////////////////////////////////////////////////

type MarkersByRank struct {
	markersByRank map[uint64]Markers
	lowestRank    uint64
	highestRank   uint64
	size          uint64
}

func NewMarkersByRank() *MarkersByRank {
	return &MarkersByRank{
		markersByRank: make(map[uint64]Markers),
		lowestRank:    1<<64 - 1,
		highestRank:   0,
		size:          0,
	}
}

func (m *MarkersByRank) Add(rank uint64, sequenceID SequenceID, index Index) (updated bool, added bool) {
	if _, exists := m.markersByRank[rank]; !exists {
		m.markersByRank[rank] = make(Markers)

		if rank > m.highestRank {
			m.highestRank = rank
		}
		if rank < m.lowestRank {
			m.lowestRank = rank
		}
	}

	updated, added = m.markersByRank[rank].Set(sequenceID, index)
	if added {
		m.size++
	}

	return
}

func (m *MarkersByRank) Markers(optionalRank ...uint64) (markers Markers, exists bool) {
	if len(optionalRank) >= 1 {
		markers, exists = m.markersByRank[optionalRank[0]]
		return
	}

	markers = make(Markers)
	for _, markersOfRank := range m.markersByRank {
		for sequenceID, index := range markersOfRank {
			markers[sequenceID] = index
		}
	}
	exists = len(markers) >= 1

	return
}

func (m *MarkersByRank) Index(rank uint64, sequenceID SequenceID) (index Index, exists bool) {
	uniqueMarkers, exists := m.markersByRank[rank]
	if !exists {
		return
	}

	index, exists = uniqueMarkers[sequenceID]

	return
}

func (m *MarkersByRank) Delete(rank uint64, sequenceID SequenceID) (deleted bool) {
	if sequences, sequencesExist := m.markersByRank[rank]; sequencesExist {
		if _, indexExists := sequences[sequenceID]; indexExists {
			delete(sequences, sequenceID)
			m.size--
			deleted = true

			if len(sequences) == 0 {
				delete(m.markersByRank, rank)

				if rank == m.lowestRank {
					if rank == m.highestRank {
						m.lowestRank = 1<<64 - 1
						m.highestRank = 0
						return
					}

					for lowestRank := m.lowestRank + 1; lowestRank <= m.highestRank; lowestRank++ {
						if _, rankExists := m.markersByRank[lowestRank]; rankExists {
							m.lowestRank = lowestRank
							break
						}
					}
				}

				if rank == m.highestRank {
					for highestRank := m.highestRank - 1; highestRank >= m.lowestRank; highestRank-- {
						if _, rankExists := m.markersByRank[highestRank]; rankExists {
							m.highestRank = highestRank
							break
						}
					}
				}
			}
		}
	}

	return
}

func (m *MarkersByRank) LowestRank() uint64 {
	return m.lowestRank
}

func (m *MarkersByRank) HighestRank() uint64 {
	return m.highestRank
}

func (m *MarkersByRank) Size() uint64 {
	return m.size
}

func (m *MarkersByRank) Clone() *MarkersByRank {
	markersByRank := make(map[uint64]Markers)
	for rank, uniqueMarkers := range m.markersByRank {
		markersByRank[rank] = uniqueMarkers.Clone()
	}

	return &MarkersByRank{
		markersByRank: markersByRank,
		lowestRank:    m.lowestRank,
		highestRank:   m.highestRank,
		size:          m.size,
	}
}

func (m *MarkersByRank) String() string {
	structBuilder := stringify.StructBuilder("MarkersByRank")
	if m.highestRank == 0 {
		return structBuilder.String()
	}

	for rank := m.lowestRank; rank <= m.highestRank; rank++ {
		if uniqueMarkers, uniqueMarkersExist := m.markersByRank[rank]; uniqueMarkersExist {
			structBuilder.AddField(stringify.StructField(strconv.FormatUint(rank, 10), uniqueMarkers))
		}
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
