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

// region NormalizedMarkers ////////////////////////////////////////////////////////////////////////////////////////////

// NormalizedMarkers represents a collection of Markers that can contain exactly one Index per SequenceID.
type NormalizedMarkers map[SequenceID]Index

func NewNormalizedMarkers(markers ...*Marker) (normalizedMarkers NormalizedMarkers) {
	normalizedMarkers = make(NormalizedMarkers)
	for _, marker := range markers {
		normalizedMarkers.Set(marker.sequenceID, marker.index)
	}

	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and added.
func (n NormalizedMarkers) Set(sequenceID SequenceID, index Index) (updated bool, added bool) {
	if existingIndex, indexAlreadyStored := n[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			n[sequenceID] = index
		}
		return
	}

	n[sequenceID] = index
	updated = true
	added = true

	return
}

func (n NormalizedMarkers) HighestIndex() (highestMarker Index) {
	for _, index := range n {
		if index > highestMarker {
			highestMarker = index
		}
	}

	return
}

// SequenceIDs returns the SequenceIDs that the normalized Markers represent.
func (n NormalizedMarkers) SequenceIDs() SequenceIDs {
	sequenceIDs := make([]SequenceID, 0, len(n))
	for sequenceID := range n {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}

	return NewSequenceIDs(sequenceIDs...)
}

// Clone create a copy of the NormalizedMarkers.
func (n NormalizedMarkers) Clone() (clone NormalizedMarkers) {
	clone = make(NormalizedMarkers)
	for sequenceID, index := range n {
		clone[sequenceID] = index
	}

	return
}

// String returns a human readable version of the NormalizedMarkers.
func (n NormalizedMarkers) String() string {
	structBuilder := stringify.StructBuilder("NormalizedMarkers")
	for sequenceID, index := range n {
		structBuilder.AddField(stringify.StructField(sequenceID.String(), index))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region NormalizedMarkersByRank //////////////////////////////////////////////////////////////////////////////////////

type NormalizedMarkersByRank struct {
	normalizedMarkersByRank map[uint64]NormalizedMarkers
	lowestRank              uint64
	highestRank             uint64
	size                    uint64
}

func NewNormalizedMarkersByRank() *NormalizedMarkersByRank {
	return &NormalizedMarkersByRank{
		normalizedMarkersByRank: make(map[uint64]NormalizedMarkers),
		lowestRank:              1<<64 - 1,
		highestRank:             0,
		size:                    0,
	}
}

func (n *NormalizedMarkersByRank) Add(rank uint64, sequenceID SequenceID, index Index) (updated bool, added bool) {
	if _, exists := n.normalizedMarkersByRank[rank]; !exists {
		n.normalizedMarkersByRank[rank] = make(NormalizedMarkers)

		if rank > n.highestRank {
			n.highestRank = rank
		}
		if rank < n.lowestRank {
			n.lowestRank = rank
		}
	}

	updated, added = n.normalizedMarkersByRank[rank].Set(sequenceID, index)
	if added {
		n.size++
	}

	return
}

func (n *NormalizedMarkersByRank) NormalizedMarkers(optionalRank ...uint64) (normalizedMarkers NormalizedMarkers, exists bool) {
	if len(optionalRank) >= 1 {
		normalizedMarkers, exists = n.normalizedMarkersByRank[optionalRank[0]]
		return
	}

	normalizedMarkers = make(NormalizedMarkers)
	for _, uniqueMarkersOfRank := range n.normalizedMarkersByRank {
		for sequenceID, index := range uniqueMarkersOfRank {
			normalizedMarkers[sequenceID] = index
		}
	}
	exists = len(normalizedMarkers) >= 1

	return
}

func (n *NormalizedMarkersByRank) Index(rank uint64, sequenceID SequenceID) (index Index, exists bool) {
	uniqueMarkers, exists := n.normalizedMarkersByRank[rank]
	if !exists {
		return
	}

	index, exists = uniqueMarkers[sequenceID]

	return
}

func (n *NormalizedMarkersByRank) Delete(rank uint64, sequenceID SequenceID) (deleted bool) {
	if sequences, sequencesExist := n.normalizedMarkersByRank[rank]; sequencesExist {
		if _, indexExists := sequences[sequenceID]; indexExists {
			delete(sequences, sequenceID)
			n.size--
			deleted = true

			if len(sequences) == 0 {
				delete(n.normalizedMarkersByRank, rank)

				if rank == n.lowestRank {
					if rank == n.highestRank {
						n.lowestRank = 1<<64 - 1
						n.highestRank = 0
						return
					}

					for lowestRank := n.lowestRank + 1; lowestRank <= n.highestRank; lowestRank++ {
						if _, rankExists := n.normalizedMarkersByRank[lowestRank]; rankExists {
							n.lowestRank = lowestRank
							break
						}
					}
				}

				if rank == n.highestRank {
					for highestRank := n.highestRank - 1; highestRank >= n.lowestRank; highestRank-- {
						if _, rankExists := n.normalizedMarkersByRank[highestRank]; rankExists {
							n.highestRank = highestRank
							break
						}
					}
				}
			}
		}
	}

	return
}

func (n *NormalizedMarkersByRank) LowestRank() uint64 {
	return n.lowestRank
}

func (n *NormalizedMarkersByRank) HighestRank() uint64 {
	return n.highestRank
}

func (n *NormalizedMarkersByRank) Size() uint64 {
	return n.size
}

func (n *NormalizedMarkersByRank) Clone() *NormalizedMarkersByRank {
	normalizedMarkersByRank := make(map[uint64]NormalizedMarkers)
	for rank, uniqueMarkers := range n.normalizedMarkersByRank {
		normalizedMarkersByRank[rank] = uniqueMarkers.Clone()
	}

	return &NormalizedMarkersByRank{
		normalizedMarkersByRank: normalizedMarkersByRank,
		lowestRank:              n.lowestRank,
		highestRank:             n.highestRank,
		size:                    n.size,
	}
}

func (n *NormalizedMarkersByRank) String() string {
	structBuilder := stringify.StructBuilder("NormalizedMarkersByRank")
	if n.highestRank == 0 {
		return structBuilder.String()
	}

	for rank := n.lowestRank; rank <= n.highestRank; rank++ {
		if uniqueMarkers, uniqueMarkersExist := n.normalizedMarkersByRank[rank]; uniqueMarkersExist {
			structBuilder.AddField(stringify.StructField(strconv.FormatUint(rank, 10), uniqueMarkers))
		}
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
