package marker

import (
	"strconv"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Index represents the ever increasing number of the Markers in a Sequence.
type Index uint64

// IndexFromBytes unmarshals an Index from a sequence of bytes.
func IndexFromBytes(sequenceBytes []byte) (index Index, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if index, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Index from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// IndexFromMarshalUtil unmarshals an Index using a MarshalUtil (for easier unmarshaling).
func IndexFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (index Index, err error) {
	untypedIndex, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Index (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	index = Index(untypedIndex)

	return
}

// Bytes returns a marshaled version of the Index.
func (i Index) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint64Size).
		WriteUint64(uint64(i)).
		Bytes()
}

// String returns a human readable version of the Index.
func (i Index) String() string {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Marker represents a point in a Sequence that is identifier by an ever increasing Index.
type Marker struct {
	sequenceID SequenceID
	index      Index
}

// New is the constructor of a Marker.
func New(sequenceID SequenceID, index Index) *Marker {
	return &Marker{
		sequenceID: sequenceID,
		index:      index,
	}
}

// FromBytes unmarshals a Marker from a sequence of bytes.
func FromBytes(markerBytes []byte) (marker *Marker, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markerBytes)
	if marker, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// FromMarshalUtil unmarshals a Marker using a MarshalUtil (for easier unmarshaling).
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

// SequenceID returns the identifier of the Sequence of the Marker.
func (m *Marker) SequenceID() SequenceID {
	return m.sequenceID
}

// Index returns the ever increasing number of the Marker in a Sequence.
func (m *Marker) Index() Index {
	return m.index
}

// Bytes returns a marshaled version of the Marker.
func (m *Marker) Bytes() []byte {
	return marshalutil.New(marshalutil.Uint64Size + marshalutil.Uint64Size).
		Write(m.sequenceID).
		Write(m.index).
		Bytes()
}

// String returns a human readable version of the Marker.
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

// NewMarkers creates a new collection of Markers from the given parameters.
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

// LowestIndex returns the the lowest Index of all Markers in the collection.
func (m Markers) LowestIndex() (lowestIndex Index) {
	lowestIndex = 1<<64 - 1
	for _, index := range m {
		if index < lowestIndex {
			lowestIndex = index
		}
	}

	return
}

// HighestIndex returns the the highest Index of all Markers in the collection.
func (m Markers) HighestIndex() (highestIndex Index) {
	for _, index := range m {
		if index > highestIndex {
			highestIndex = index
		}
	}

	return
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

// MarkersByRank is a collection of Markers that groups them by the rank of their Sequence.
type MarkersByRank struct {
	markersByRank map[uint64]Markers
	lowestRank    uint64
	highestRank   uint64
	size          uint64
}

// NewMarkersByRank creates a new collection of Markers grouped by the rank of their Sequence.
func NewMarkersByRank() *MarkersByRank {
	return &MarkersByRank{
		markersByRank: make(map[uint64]Markers),
		lowestRank:    1<<64 - 1,
		highestRank:   0,
		size:          0,
	}
}

// Add adds a new Marker to the collection and returns if the marker was updated
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

// Markers flattens the collection and returns a normal Markers collection by removing the rank information. The
// optionalRank parameter allows to optionally filter the collection by rank and only return the Markers of the given
// rank. The method additionally returns an exists flag that indicates if the returned Markers are not empty.
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

// Index returns the Index of the Marker given by the rank and its SequenceID and a flag that indicates if the Marker
// exists in the collection.
func (m *MarkersByRank) Index(rank uint64, sequenceID SequenceID) (index Index, exists bool) {
	uniqueMarkers, exists := m.markersByRank[rank]
	if !exists {
		return
	}

	index, exists = uniqueMarkers[sequenceID]

	return
}

// Delete removes the given Marker from the collection and returns a flag that indicates if the Marker existed in the
// collection.
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

// LowestRank returns the lowest rank that has Markers.
func (m *MarkersByRank) LowestRank() uint64 {
	return m.lowestRank
}

// HighestRank returns the highest rank that has Markers.
func (m *MarkersByRank) HighestRank() uint64 {
	return m.highestRank
}

// Size returns the amount of Markers in the collection.
func (m *MarkersByRank) Size() uint64 {
	return m.size
}

// Clone returns a copy of the MarkersByRank.
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

// String returns a human readable version of the MarkersByRank.
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
