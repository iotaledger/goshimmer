package markers

import (
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Marker represents a coordinate in a Sequence that is identified by an ever increasing Index.
type Marker struct {
	sequenceID SequenceID
	index      Index
}

// MarkerFromBytes unmarshals a Marker from a sequence of bytes.
func MarkerFromBytes(markerBytes []byte) (marker *Marker, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markerBytes)
	if marker, err = MarkerFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Marker from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// MarkerFromMarshalUtil unmarshals a Marker using a MarshalUtil (for easier unmarshaling).
func MarkerFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (marker *Marker, err error) {
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
func (m *Marker) SequenceID() (sequenceID SequenceID) {
	return m.sequenceID
}

// Index returns the coordinate of the Marker in a Sequence.
func (m *Marker) Index() (index Index) {
	return m.index
}

// Bytes returns a marshaled version of the Marker.
func (m *Marker) Bytes() (marshaledMarker []byte) {
	return marshalutil.New(marshalutil.Uint64Size + marshalutil.Uint64Size).
		Write(m.sequenceID).
		Write(m.index).
		Bytes()
}

// String returns a human readable version of the Marker.
func (m *Marker) String() (humanReadableMarker string) {
	return stringify.Struct("Marker",
		stringify.StructField("sequenceID", m.SequenceID()),
		stringify.StructField("index", m.Index()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	markers      map[SequenceID]Index
	highestIndex Index
	lowestIndex  Index
	markersMutex sync.RWMutex
}

// FromBytes unmarshals a collection of Markers from a sequence of bytes.
func FromBytes(markersBytes []byte) (markers *Markers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(markersBytes)
	if markers, err = FromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Markers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromMarshalUtil unmarshals a collection of Markers using a MarshalUtil (for easier unmarshaling).
func FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (markers *Markers, err error) {
	markersCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse Markers count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	markers = &Markers{
		markers: make(map[SequenceID]Index),
	}
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
		markers.Set(sequenceID, index)
	}

	return
}

// NewMarkers creates a new collection of Markers.
func NewMarkers(optionalMarkers ...*Marker) (markers *Markers) {
	markers = &Markers{
		markers: make(map[SequenceID]Index),
	}
	for _, marker := range optionalMarkers {
		markers.Set(marker.sequenceID, marker.index)
	}

	return
}

// SequenceIDs returns the SequenceIDs that are having Markers in this collection.
func (m *Markers) SequenceIDs() (sequenceIDs SequenceIDs) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	sequenceIDsSlice := make([]SequenceID, 0, len(m.markers))
	for sequenceID := range m.markers {
		sequenceIDsSlice = append(sequenceIDsSlice, sequenceID)
	}

	return NewSequenceIDs(sequenceIDsSlice...)
}

// FirstMarker returns the first Marker in the collection. It can for example be used to retrieve the new Marker that
// was assigned when increasing the Index of a Sequence.
func (m *Markers) FirstMarker() (firstMarker *Marker) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	for sequenceID, index := range m.markers {
		firstMarker = &Marker{sequenceID: sequenceID, index: index}
		return
	}

	return
}

// Get returns the Index of the Marker with the given Sequence and a flag that indicates if the Marker exists.
func (m *Markers) Get(sequenceID SequenceID) (index Index, exists bool) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	index, exists = m.markers[sequenceID]
	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and/or added.
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated bool, added bool) {
	m.markersMutex.Lock()
	defer m.markersMutex.Unlock()

	if index > m.highestIndex {
		m.highestIndex = index
	}

	if existingIndex, indexAlreadyStored := m.markers[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m.markers[sequenceID] = index

			if index == m.lowestIndex {
				m.lowestIndex = 0
				for _, scannedIndex := range m.markers {
					if scannedIndex < m.lowestIndex || m.lowestIndex == 0 {
						m.lowestIndex = scannedIndex
					}
				}
			}
		}

		return
	}

	if index < m.lowestIndex || m.lowestIndex == 0 {
		m.lowestIndex = index
	}

	m.markers[sequenceID] = index
	updated = true
	added = true

	return
}

// Delete removes the Marker with the given SequenceID from the collection and returns a boolean flag that indicates if
// the element existed.
func (m *Markers) Delete(sequenceID SequenceID) (existed bool) {
	m.markersMutex.Lock()
	defer m.markersMutex.Unlock()

	existingIndex, existed := m.markers[sequenceID]
	delete(m.markers, sequenceID)
	if existed {
		lowestIndexDeleted := existingIndex == m.lowestIndex
		if lowestIndexDeleted {
			m.lowestIndex = 0
		}
		highestIndexDeleted := existingIndex == m.highestIndex
		if highestIndexDeleted {
			m.highestIndex = 0
		}

		if lowestIndexDeleted || highestIndexDeleted {
			for _, scannedIndex := range m.markers {
				if scannedIndex < m.lowestIndex || m.lowestIndex == 0 {
					m.lowestIndex = scannedIndex
				}
				if scannedIndex > m.highestIndex {
					m.highestIndex = scannedIndex
				}
			}
		}
	}

	return
}

// ForEach calls the iterator for each of the contained Markers. The iteration is aborted if the iterator returns false.
// The method returns false if the iteration was aborted.
func (m *Markers) ForEach(iterator func(sequenceID SequenceID, index Index) bool) (success bool) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	success = true
	for sequenceID, index := range m.markers {
		if success = iterator(sequenceID, index); !success {
			return
		}
	}

	return
}

// Size returns the amount of Markers in the collection.
func (m *Markers) Size() (size int) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	return len(m.markers)
}

// Merge takes the given Markers and adds them to the collection (overwriting Markers with a lower Index if there are
// existing Markers with the same SequenceID).
func (m *Markers) Merge(markers *Markers) {
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		m.Set(sequenceID, index)

		return true
	})
}

// LowestIndex returns the the lowest Index of all Markers in the collection.
func (m *Markers) LowestIndex() (lowestIndex Index) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	lowestIndex = m.lowestIndex

	return
}

// HighestIndex returns the the highest Index of all Markers in the collection.
func (m *Markers) HighestIndex() (highestIndex Index) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	highestIndex = m.highestIndex

	return
}

// Clone creates a deep copy of the Markers.
func (m *Markers) Clone() (clonedMarkers *Markers) {
	clonedMap := make(map[SequenceID]Index)
	m.ForEach(func(sequenceID SequenceID, index Index) bool {
		clonedMap[sequenceID] = index

		return true
	})

	clonedMarkers = &Markers{
		markers:      clonedMap,
		lowestIndex:  m.lowestIndex,
		highestIndex: m.highestIndex,
	}

	return
}

// Bytes returns the Markers in serialized byte form.
func (m *Markers) Bytes() (marshalMarkers []byte) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(m.markers)))
	for sequenceID, index := range m.markers {
		marshalUtil.Write(sequenceID)
		marshalUtil.Write(index)
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the Markers.
func (m *Markers) String() (humanReadableMarkers string) {
	structBuilder := stringify.StructBuilder("Markers")
	m.ForEach(func(sequenceID SequenceID, index Index) bool {
		structBuilder.AddField(stringify.StructField(sequenceID.String(), index))

		return true
	})
	structBuilder.AddField(stringify.StructField("lowestIndex", m.LowestIndex()))
	structBuilder.AddField(stringify.StructField("highestIndex", m.HighestIndex()))

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region markersByRank ////////////////////////////////////////////////////////////////////////////////////////////////

// markersByRank is a collection of Markers that groups them by the rank of their Sequence.
type markersByRank struct {
	markersByRank      map[uint64]*Markers
	markersByRankMutex sync.RWMutex
	lowestRank         uint64
	highestRank        uint64
	size               uint64
}

// newMarkersByRank creates a new collection of Markers grouped by the rank of their Sequence.
func newMarkersByRank() (newMarkersByRank *markersByRank) {
	return &markersByRank{
		markersByRank: make(map[uint64]*Markers),
		lowestRank:    1<<64 - 1,
		highestRank:   0,
		size:          0,
	}
}

// Add adds a new Marker to the collection and returns two boolean flags that indicate if a Marker was added and/or
// updated.
func (m *markersByRank) Add(rank uint64, sequenceID SequenceID, index Index) (updated bool, added bool) {
	m.markersByRankMutex.Lock()
	defer m.markersByRankMutex.Unlock()

	if _, exists := m.markersByRank[rank]; !exists {
		m.markersByRank[rank] = NewMarkers()

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
// rank. The method additionally returns an exists flag that indicates if the returned Markers contain at least one
// element.
func (m *markersByRank) Markers(optionalRank ...uint64) (markers *Markers, exists bool) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	if len(optionalRank) >= 1 {
		markers, exists = m.markersByRank[optionalRank[0]]
		return
	}

	markers = NewMarkers()
	for _, markersOfRank := range m.markersByRank {
		markersOfRank.ForEach(func(sequenceID SequenceID, index Index) bool {
			markers.Set(sequenceID, index)

			return true
		})
	}
	exists = markers.Size() >= 1

	return
}

// Index returns the Index of the Marker given by the rank and its SequenceID and a flag that indicates if the Marker
// exists in the collection.
func (m *markersByRank) Index(rank uint64, sequenceID SequenceID) (index Index, exists bool) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	uniqueMarkers, exists := m.markersByRank[rank]
	if !exists {
		return
	}

	index, exists = uniqueMarkers.Get(sequenceID)

	return
}

// Delete removes the given Marker from the collection and returns a flag that indicates if the Marker existed in the
// collection.
func (m *markersByRank) Delete(rank uint64, sequenceID SequenceID) (deleted bool) {
	m.markersByRankMutex.Lock()
	defer m.markersByRankMutex.Unlock()

	if sequences, sequencesExist := m.markersByRank[rank]; sequencesExist {
		if deleted = sequences.Delete(sequenceID); deleted {
			m.size--

			if sequences.Size() == 0 {
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
func (m *markersByRank) LowestRank() (lowestRank uint64) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	return m.lowestRank
}

// HighestRank returns the highest rank that has Markers.
func (m *markersByRank) HighestRank() (highestRank uint64) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	return m.highestRank
}

// Size returns the amount of Markers in the collection.
func (m *markersByRank) Size() (size uint64) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	return m.size
}

// Clone returns a deep copy of the markersByRank.
func (m *markersByRank) Clone() (clonedMarkersByRank *markersByRank) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	markersByRankMap := make(map[uint64]*Markers)
	for rank, uniqueMarkers := range m.markersByRank {
		markersByRankMap[rank] = uniqueMarkers.Clone()
	}

	return &markersByRank{
		markersByRank: markersByRankMap,
		lowestRank:    m.lowestRank,
		highestRank:   m.highestRank,
		size:          m.size,
	}
}

// String returns a human readable version of the markersByRank.
func (m *markersByRank) String() (humanReadableMarkersByRank string) {
	m.markersByRankMutex.RLock()
	defer m.markersByRankMutex.RUnlock()

	structBuilder := stringify.StructBuilder("markersByRank")
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
