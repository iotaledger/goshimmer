package markers

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// MarkerLength represents the amount of bytes of a marshaled Marker.
const MarkerLength = SequenceIDLength + IndexLength

// Marker represents a coordinate in a Sequence that is identified by an ever increasing Index.
type Marker struct {
	sequenceID SequenceID
	index      Index
}

// NewMarker returns a new marker.
func NewMarker(sequenceID SequenceID, index Index) *Marker {
	return &Marker{sequenceID, index}
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
func (m Marker) Bytes() (marshaledMarker []byte) {
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

// Marker type casts the Markers to a Marker if it contains only 1 element.
func (m *Markers) Marker() (marker *Marker) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	switch len(m.markers) {
	case 0:
		panic("converting empty Markers into a single Marker is not supported")
	case 1:
		for sequenceID, index := range m.markers {
			return &Marker{sequenceID: sequenceID, index: index}
		}
	default:
		panic("converting multiple Markers into a single Marker is not supported")
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
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated, added bool) {
	m.markersMutex.Lock()
	defer m.markersMutex.Unlock()

	if index > m.highestIndex {
		m.highestIndex = index
	}

	// if the sequence already exists in the set and the new index is higher than the old one then update
	if existingIndex, indexAlreadyStored := m.markers[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m.markers[sequenceID] = index

			// find new lowest index
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

	// if this is a new sequence update lowestIndex
	if index < m.lowestIndex || m.lowestIndex == 0 {
		m.lowestIndex = index
	}

	m.markers[sequenceID] = index

	return true, true
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
	markersCopy := make(map[SequenceID]Index)
	for sequenceID, index := range m.markers {
		markersCopy[sequenceID] = index
	}
	m.markersMutex.RUnlock()

	success = true
	for sequenceID, index := range markersCopy {
		if success = iterator(sequenceID, index); !success {
			return
		}
	}

	return
}

// ForEachSorted calls the iterator for each of the contained Markers in increasing order. The iteration is aborted if
// the iterator returns false. The method returns false if the iteration was aborted.
func (m *Markers) ForEachSorted(iterator func(sequenceID SequenceID, index Index) bool) (success bool) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	sequenceIDs := make([]SequenceID, 0, len(m.markers))
	for sequenceID := range m.markers {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}
	sort.Slice(sequenceIDs, func(i, j int) bool {
		return sequenceIDs[i] < sequenceIDs[j]
	})

	success = true
	for _, sequenceID := range sequenceIDs {
		if success = iterator(sequenceID, m.markers[sequenceID]); !success {
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

// Equals is a comparator for two Markers.
func (m *Markers) Equals(other *Markers) (equals bool) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	if len(m.markers) != len(other.markers) {
		return false
	}

	for sequenceID, index := range m.markers {
		otherIndex, exists := other.markers[sequenceID]
		if !exists {
			return false
		}

		if otherIndex != index {
			return false
		}
	}

	return true
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

	return structBuilder.String()
}

// SequenceToString returns a string in the form sequenceID:index;.
func (m *Markers) SequenceToString() (s string) {
	parts := make([]string, 0, m.Size())
	m.ForEach(func(sequenceID SequenceID, index Index) bool {
		parts = append(parts, fmt.Sprintf("%d:%d", sequenceID, index))
		return true
	})
	s = strings.Join(parts, ";")
	return s
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferencingMarkers ///////////////////////////////////////////////////////////////////////////////////////////

// ReferencingMarkers is a data structure that allows to denote which Markers of child Sequences in the Sequence DAG
// reference a given Marker in a Sequence.
type ReferencingMarkers struct {
	referencingIndexesBySequence markerReferences
	mutex                        sync.RWMutex
}

// NewReferencingMarkers is the constructor for the ReferencingMarkers.
func NewReferencingMarkers() (referencingMarkers *ReferencingMarkers) {
	referencingMarkers = &ReferencingMarkers{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	return
}

// ReferencingMarkersFromBytes unmarshals ReferencingMarkers from a sequence of bytes.
func ReferencingMarkersFromBytes(referencingMarkersBytes []byte) (referencingMarkers *ReferencingMarkers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(referencingMarkersBytes)
	if referencingMarkers, err = ReferencingMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ReferencingMarkers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencingMarkersFromMarshalUtil unmarshals ReferencingMarkers using a MarshalUtil (for easier unmarshaling).
func ReferencingMarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (referencingMarkers *ReferencingMarkers, err error) {
	referencingMarkers = &ReferencingMarkers{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	referencingMarkers.referencingIndexesBySequence, err = markerReferencesFromMarshalUtil(marshalUtil, thresholdmap.UpperThresholdMode)
	return referencingMarkers, err
}

// Add adds a new referencing Marker to the ReferencingMarkers.
func (r *ReferencingMarkers) Add(index Index, referencingMarker *Marker) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	thresholdMap, thresholdMapExists := r.referencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = thresholdmap.New(thresholdmap.UpperThresholdMode)
		r.referencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(index), referencingMarker.Index())
}

// Get returns the Markers of child Sequences that reference the given Index.
func (r *ReferencingMarkers) Get(index Index) (referencingMarkers *Markers) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		if referencingIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencingMarkers.Set(sequenceID, referencingIndex.(Index))
		}
	}

	return
}

// Bytes returns a marshaled version of the ReferencingMarkers.
func (r *ReferencingMarkers) Bytes() (marshaledReferencingMarkers []byte) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(r.referencingIndexesBySequence)))
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		marshalUtil.Write(sequenceID)
		marshalUtil.WriteUint64(uint64(thresholdMap.Size()))
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			marshalUtil.WriteUint64(node.Key().(uint64))
			marshalUtil.WriteUint64(uint64(node.Value().(Index)))

			return true
		})
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the ReferencingMarkers.
func (r *ReferencingMarkers) String() (humanReadableReferencingMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	indexes := make([]Index, 0)
	referencingMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			index := Index(node.Key().(uint64))
			referencingIndex := node.Value().(Index)
			if _, exists := referencingMarkersByReferencingIndex[index]; !exists {
				referencingMarkersByReferencingIndex[index] = NewMarkers()

				indexes = append(indexes, index)
			}

			referencingMarkersByReferencingIndex[index].Set(sequenceID, referencingIndex)

			return true
		})
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})

	for i, index := range indexes {
		for j := i + 1; j < len(indexes); j++ {
			referencingMarkersByReferencingIndex[indexes[j]].ForEach(func(referencingSequenceID SequenceID, referencingIndex Index) bool {
				if _, exists := referencingMarkersByReferencingIndex[index].Get(referencingSequenceID); exists {
					return true
				}

				referencingMarkersByReferencingIndex[index].Set(referencingSequenceID, referencingIndex)

				return true
			})
		}
	}

	thresholdStart := "0"
	referencingMarkers := stringify.StructBuilder("ReferencingMarkers")
	for _, index := range indexes {
		thresholdEnd := strconv.FormatUint(uint64(index), 10)

		if thresholdStart == thresholdEnd {
			referencingMarkers.AddField(stringify.StructField("Index("+thresholdStart+")", referencingMarkersByReferencingIndex[index]))
		} else {
			referencingMarkers.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencingMarkersByReferencingIndex[index]))
		}

		thresholdStart = strconv.FormatUint(uint64(index)+1, 10)
	}

	return referencingMarkers.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferencedMarkers ////////////////////////////////////////////////////////////////////////////////////////////

// ReferencedMarkers is a data structure that allows to denote which Marker of a Sequence references which other Markers
// of its parent Sequences in the Sequence DAG.
type ReferencedMarkers struct {
	referencedIndexesBySequence markerReferences
	mutex                       sync.RWMutex
}

// NewReferencedMarkers is the constructor for the ReferencedMarkers.
func NewReferencedMarkers(markers *Markers) (referencedMarkers *ReferencedMarkers) {
	referencedMarkers = &ReferencedMarkers{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	initialSequenceIndex := markers.HighestIndex() + 1
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := thresholdmap.New(thresholdmap.LowerThresholdMode)
		thresholdMap.Set(uint64(initialSequenceIndex), index)

		referencedMarkers.referencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// ReferencedMarkersFromBytes unmarshals ReferencedMarkers from a sequence of bytes.
func ReferencedMarkersFromBytes(parentReferencesBytes []byte) (referencedMarkers *ReferencedMarkers, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(parentReferencesBytes)
	if referencedMarkers, err = ReferencedMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ReferencedMarkers from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReferencedMarkersFromMarshalUtil unmarshals ReferencedMarkers using a MarshalUtil (for easier unmarshaling).
func ReferencedMarkersFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (referencedMarkers *ReferencedMarkers, err error) {
	referencedMarkers = &ReferencedMarkers{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap),
	}

	referencedMarkers.referencedIndexesBySequence, err = markerReferencesFromMarshalUtil(marshalUtil, thresholdmap.LowerThresholdMode)
	return referencedMarkers, err
}

// Add adds new referenced Markers to the ReferencedMarkers.
func (r *ReferencedMarkers) Add(index Index, referencedMarkers *Markers) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := r.referencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = thresholdmap.New(thresholdmap.LowerThresholdMode)
			r.referencedIndexesBySequence[referencedSequenceID] = thresholdMap
		}

		thresholdMap.Set(uint64(index), referencedIndex)

		return true
	})
}

// Get returns the Markers of parent Sequences that were referenced by the given Index.
func (r *ReferencedMarkers) Get(index Index) (referencedMarkers *Markers) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencedMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		if referencedIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencedMarkers.Set(sequenceID, referencedIndex.(Index))
		}
	}

	return
}

// Bytes returns a marshaled version of the ReferencedMarkers.
func (r *ReferencedMarkers) Bytes() (marshaledReferencedMarkers []byte) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint64(uint64(len(r.referencedIndexesBySequence)))
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		marshalUtil.Write(sequenceID)
		marshalUtil.WriteUint64(uint64(thresholdMap.Size()))
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			marshalUtil.WriteUint64(node.Key().(uint64))
			marshalUtil.WriteUint64(uint64(node.Value().(Index)))

			return true
		})
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the ReferencedMarkers.
func (r *ReferencedMarkers) String() (humanReadableReferencedMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	indexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element) bool {
			index := Index(node.Key().(uint64))
			referencedIndex := Index(node.Value().(uint64))
			if _, exists := referencedMarkersByReferencingIndex[index]; !exists {
				referencedMarkersByReferencingIndex[index] = NewMarkers()

				indexes = append(indexes, index)
			}

			referencedMarkersByReferencingIndex[index].Set(sequenceID, referencedIndex)

			return true
		})
	}
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i] < indexes[j]
	})

	for i, referencedIndex := range indexes {
		for j := 0; j < i; j++ {
			referencedMarkersByReferencingIndex[indexes[j]].ForEach(func(sequenceID SequenceID, index Index) bool {
				if _, exists := referencedMarkersByReferencingIndex[referencedIndex].Get(sequenceID); exists {
					return true
				}

				referencedMarkersByReferencingIndex[referencedIndex].Set(sequenceID, index)

				return true
			})
		}
	}

	referencedMarkers := stringify.StructBuilder("ReferencedMarkers")
	for i, index := range indexes {
		thresholdStart := strconv.FormatUint(uint64(index), 10)
		thresholdEnd := "INF"
		if len(indexes) > i+1 {
			thresholdEnd = strconv.FormatUint(uint64(indexes[i+1])-1, 10)
		}

		if thresholdStart == thresholdEnd {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+")", referencedMarkersByReferencingIndex[index]))
		} else {
			referencedMarkers.AddField(stringify.StructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencedMarkersByReferencingIndex[index]))
		}
	}

	return referencedMarkers.String()
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
func (m *markersByRank) Add(rank uint64, sequenceID SequenceID, index Index) (updated, added bool) {
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

	return deleted
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

// region markerReferences /////////////////////////////////////////////////////////////////////////////////////////////

// markerReferences represents a type that encodes the reference between Markers of different Sequences.
type markerReferences map[SequenceID]*thresholdmap.ThresholdMap

// markerReferencesFromMarshalUtil unmarshals markerReferences using a MarshalUtil (for easier unmarshaling).
func markerReferencesFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil, mode thresholdmap.Mode) (referenceMarkers markerReferences, err error) {
	referenceMarkers = make(map[SequenceID]*thresholdmap.ThresholdMap)

	sequenceCount, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse Sequence count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	for i := uint64(0); i < sequenceCount; i++ {
		sequenceID, sequenceIDErr := SequenceIDFromMarshalUtil(marshalUtil)
		if sequenceIDErr != nil {
			err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", sequenceIDErr)
			return
		}

		referenceCount, referenceCountErr := marshalUtil.ReadUint64()
		if referenceCountErr != nil {
			err = xerrors.Errorf("failed to parse reference count (%v): %w", referenceCountErr, cerrors.ErrParseBytesFailed)
			return
		}
		thresholdMap := thresholdmap.New(mode)
		switch mode {
		case thresholdmap.LowerThresholdMode:
			for j := uint64(0); j < referenceCount; j++ {
				referencingIndex, referencingIndexErr := marshalUtil.ReadUint64()
				if referencingIndexErr != nil {
					err = xerrors.Errorf("failed to read referencing Index (%v): %w", referencingIndexErr, cerrors.ErrParseBytesFailed)
					return
				}

				referencedIndex, referencedIndexErr := marshalUtil.ReadUint64()
				if referencedIndexErr != nil {
					err = xerrors.Errorf("failed to read referenced Index (%v): %w", referencedIndexErr, cerrors.ErrParseBytesFailed)
					return
				}

				thresholdMap.Set(referencingIndex, Index(referencedIndex))
			}
		case thresholdmap.UpperThresholdMode:
			for j := uint64(0); j < referenceCount; j++ {
				referencedIndex, referencedIndexErr := marshalUtil.ReadUint64()
				if referencedIndexErr != nil {
					err = xerrors.Errorf("failed to read referenced Index (%v): %w", referencedIndexErr, cerrors.ErrParseBytesFailed)
					return
				}

				referencingIndex, referencingIndexErr := marshalUtil.ReadUint64()
				if referencingIndexErr != nil {
					err = xerrors.Errorf("failed to read referencing Index (%v): %w", referencingIndexErr, cerrors.ErrParseBytesFailed)
					return
				}

				thresholdMap.Set(referencedIndex, Index(referencingIndex))
			}
		}

		referenceMarkers[sequenceID] = thresholdMap
	}

	return referenceMarkers, err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
