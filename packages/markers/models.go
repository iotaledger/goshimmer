package markers

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/thresholdmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// IndexLength represents the amount of bytes of a marshaled Index.
const IndexLength = marshalutil.Uint64Size

// Index represents the ever-increasing number of the Markers in a Sequence.
type Index uint64

// String returns a human-readable version of the Index.
func (i Index) String() (humanReadableIndex string) {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IncreaseIndexCallback ////////////////////////////////////////////////////////////////////////////////////////

// IncreaseIndexCallback is the type of the callback function that is used to determine if a new Index is supposed to be
// assigned in a given Sequence.
type IncreaseIndexCallback func(sequenceID SequenceID, currentHighestIndex Index) bool

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IndexComparator //////////////////////////////////////////////////////////////////////////////////////////////

// IndexComparator is a generic comparator for Index types.
func IndexComparator(a, b interface{}) int {
	aCasted := a.(Index)
	bCasted := b.(Index)
	switch {
	case aCasted < bCasted:
		return -1
	case aCasted > bCasted:
		return 1
	default:
		return 0
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// MarkerLength represents the amount of bytes of a marshaled Marker.
const MarkerLength = SequenceIDLength + IndexLength

// Marker represents a coordinate in a Sequence that is identified by an ever-increasing Index.
type Marker struct {
	markerInner `serix:"0"`
}

type markerInner struct {
	SequenceID SequenceID `serix:"0"`
	Index      Index      `serix:"1"`
}

// NewMarker returns a new marker.
func NewMarker(sequenceID SequenceID, index Index) *Marker {
	return &Marker{markerInner{sequenceID, index}}
}

// MarkerFromBytes unmarshals Marker from a sequence of bytes.
func MarkerFromBytes(data []byte) (marker *Marker, consumedBytes int, err error) {
	marker = new(Marker)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, marker, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Marker: %w", err)
		return
	}
	return
}

// SequenceID returns the identifier of the Sequence of the Marker.
func (m *Marker) SequenceID() (sequenceID SequenceID) {
	return m.markerInner.SequenceID
}

// Index returns the coordinate of the Marker in a Sequence.
func (m *Marker) Index() (index Index) {
	return m.markerInner.Index
}

// Bytes returns a marshaled version of the Marker.
func (m Marker) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the Marker.
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
	markersInner `serix:"0"`
}

type markersInner struct {
	Markers      map[SequenceID]Index `serix:"0,lengthPrefixType=uint32"`
	HighestIndex Index
	LowestIndex  Index
	markersMutex sync.RWMutex
}

// FromBytes unmarshals a collection of Markers from a sequence of bytes.
func FromBytes(data []byte) (markers *Markers, consumedBytes int, err error) {
	markersDecoded := new(Markers)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), data, markersDecoded, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Markers: %w", err)
		return
	}
	markers = NewMarkers()
	for sequenceID, index := range markersDecoded.Markers {
		markers.Set(sequenceID, index)
	}
	return
}

// NewMarkers creates a new collection of Markers.
func NewMarkers(optionalMarkers ...*Marker) (markers *Markers) {
	markers = &Markers{
		markersInner{
			Markers: make(map[SequenceID]Index),
		},
	}
	for _, marker := range optionalMarkers {
		markers.Set(marker.markerInner.SequenceID, marker.markerInner.Index)
	}

	return
}

// SequenceIDs returns the SequenceIDs that are having Markers in this collection.
func (m *Markers) SequenceIDs() (sequenceIDs SequenceIDs) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	sequenceIDsSlice := make([]SequenceID, 0, len(m.markersInner.Markers))
	for sequenceID := range m.markersInner.Markers {
		sequenceIDsSlice = append(sequenceIDsSlice, sequenceID)
	}

	return NewSequenceIDs(sequenceIDsSlice...)
}

// Marker type casts the Markers to a Marker if it contains only 1 element.
func (m *Markers) Marker() (marker *Marker) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	switch len(m.markersInner.Markers) {
	case 0:
		panic("converting empty Markers into a single Marker is not supported")
	case 1:
		for sequenceID, index := range m.markersInner.Markers {
			return &Marker{markerInner{SequenceID: sequenceID, Index: index}}
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

	index, exists = m.markersInner.Markers[sequenceID]
	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and/or added.
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated, added bool) {
	m.markersMutex.Lock()
	defer m.markersMutex.Unlock()

	if index > m.markersInner.HighestIndex {
		m.markersInner.HighestIndex = index
	}

	// if the sequence already exists in the set and the new index is higher than the old one then update
	if existingIndex, indexAlreadyStored := m.markersInner.Markers[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m.markersInner.Markers[sequenceID] = index

			// find new lowest index
			if existingIndex == m.markersInner.LowestIndex {
				m.markersInner.LowestIndex = 0
				for _, scannedIndex := range m.markersInner.Markers {
					if scannedIndex < m.markersInner.LowestIndex || m.markersInner.LowestIndex == 0 {
						m.markersInner.LowestIndex = scannedIndex
					}
				}
			}
		}

		return
	}

	// if this is a new sequence update lowestIndex
	if index < m.markersInner.LowestIndex || m.markersInner.LowestIndex == 0 {
		m.markersInner.LowestIndex = index
	}

	m.markersInner.Markers[sequenceID] = index

	return true, true
}

// Delete removes the Marker with the given SequenceID from the collection and returns a boolean flag that indicates if
// the element existed.
func (m *Markers) Delete(sequenceID SequenceID) (existed bool) {
	m.markersMutex.Lock()
	defer m.markersMutex.Unlock()

	existingIndex, existed := m.markersInner.Markers[sequenceID]
	delete(m.markersInner.Markers, sequenceID)
	if existed {
		lowestIndexDeleted := existingIndex == m.markersInner.LowestIndex
		if lowestIndexDeleted {
			m.markersInner.LowestIndex = 0
		}
		highestIndexDeleted := existingIndex == m.markersInner.HighestIndex
		if highestIndexDeleted {
			m.markersInner.HighestIndex = 0
		}

		if lowestIndexDeleted || highestIndexDeleted {
			for _, scannedIndex := range m.markersInner.Markers {
				if scannedIndex < m.markersInner.LowestIndex || m.markersInner.LowestIndex == 0 {
					m.markersInner.LowestIndex = scannedIndex
				}
				if scannedIndex > m.markersInner.HighestIndex {
					m.markersInner.HighestIndex = scannedIndex
				}
			}
		}
	}

	return
}

// ForEach calls the iterator for each of the contained Markers. The iteration is aborted if the iterator returns false.
// The method returns false if the iteration was aborted.
func (m *Markers) ForEach(iterator func(sequenceID SequenceID, index Index) bool) (success bool) {
	if m == nil {
		return true
	}
	m.markersMutex.RLock()
	markersCopy := make(map[SequenceID]Index)
	for sequenceID, index := range m.markersInner.Markers {
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
	clonedMarkers := m.Clone().markersInner.Markers

	sequenceIDs := make([]SequenceID, 0, len(clonedMarkers))
	for sequenceID := range clonedMarkers {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}
	sort.Slice(sequenceIDs, func(i, j int) bool {
		return sequenceIDs[i] > sequenceIDs[j]
	})

	success = true
	for _, sequenceID := range sequenceIDs {
		if success = iterator(sequenceID, clonedMarkers[sequenceID]); !success {
			return
		}
	}

	return
}

// Size returns the amount of Markers in the collection.
func (m *Markers) Size() (size int) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	return len(m.markersInner.Markers)
}

// Merge takes the given Markers and adds them to the collection (overwriting Markers with a lower Index if there are
// existing Markers with the same SequenceID).
func (m *Markers) Merge(markers *Markers) {
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		m.Set(sequenceID, index)

		return true
	})
}

// LowestIndex returns the lowest Index of all Markers in the collection.
func (m *Markers) LowestIndex() (lowestIndex Index) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	lowestIndex = m.markersInner.LowestIndex

	return
}

// HighestIndex returns the highest Index of all Markers in the collection.
func (m *Markers) HighestIndex() (highestIndex Index) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	highestIndex = m.markersInner.HighestIndex

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
		markersInner{
			Markers:      clonedMap,
			LowestIndex:  m.markersInner.LowestIndex,
			HighestIndex: m.markersInner.HighestIndex,
		},
	}

	return
}

// Equals is a comparator for two Markers.
func (m *Markers) Equals(other *Markers) (equals bool) {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()

	if len(m.markersInner.Markers) != len(other.markersInner.Markers) {
		return false
	}

	for sequenceID, index := range m.markersInner.Markers {
		otherIndex, exists := other.markersInner.Markers[sequenceID]
		if !exists {
			return false
		}

		if otherIndex != index {
			return false
		}
	}

	return true
}

// Bytes returns a marshaled version of the Markers.
func (m *Markers) Bytes() []byte {
	m.markersMutex.RLock()
	defer m.markersMutex.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the Markers.
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
	referencingMarkersInner `serix:"0"`
}

type referencingMarkersInner struct {
	ReferencingIndexesBySequence map[SequenceID]*referencingMarkersMap `serix:"0,lengthPrefixType=uint32"`
	mutex                        sync.RWMutex
}

// NewReferencingMarkers is the constructor for the ReferencingMarkers.
func NewReferencingMarkers() (referencingMarkers *ReferencingMarkers) {
	referencingMarkers = &ReferencingMarkers{
		referencingMarkersInner{
			ReferencingIndexesBySequence: make(map[SequenceID]*referencingMarkersMap),
		},
	}

	return
}

// ReferencingMarkersFromBytes unmarshals ReferencingMarkers from a sequence of bytes.
func ReferencingMarkersFromBytes(referencingMarkersBytes []byte) (referencingMarkers *ReferencingMarkers, consumedBytes int, err error) {
	referencingMarkers = new(ReferencingMarkers)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), referencingMarkersBytes, referencingMarkers, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ReferencingMarkers: %w", err)
		return
	}
	return
}

// Add adds a new referencing Marker to the ReferencingMarkers.
func (r *ReferencingMarkers) Add(index Index, referencingMarker *Marker) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	thresholdMap, thresholdMapExists := r.ReferencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = newReferencingMarkersMap()
		r.ReferencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(index), referencingMarker.Index())
}

// Get returns the Markers of child Sequences that reference the given Index.
func (r *ReferencingMarkers) Get(index Index) (referencingMarkers *Markers) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.ReferencingIndexesBySequence {
		if referencingIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencingMarkers.Set(sequenceID, referencingIndex)
		}
	}

	return
}

// Bytes returns a marshaled version of the PersistableBaseMana.
func (r *ReferencingMarkers) Bytes() []byte {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	objBytes, err := serix.DefaultAPI.Encode(context.Background(), r)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the ReferencingMarkers.
func (r *ReferencingMarkers) String() (humanReadableReferencingMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	indexes := make([]Index, 0)
	referencingMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.ReferencingIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element[uint64, Index]) bool {
			index := Index(node.Key())
			referencingIndex := node.Value()
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
	referencedMarkersInner `serix:"0"`
}
type referencedMarkersInner struct {
	ReferencedIndexesBySequence map[SequenceID]*referencedMarkersMap `serix:"0,lengthPrefixType=uint32"`
	mutex                       sync.RWMutex
}

// NewReferencedMarkers is the constructor for the ReferencedMarkers.
func NewReferencedMarkers(markers *Markers) (referencedMarkers *ReferencedMarkers) {
	referencedMarkers = &ReferencedMarkers{
		referencedMarkersInner{
			ReferencedIndexesBySequence: make(map[SequenceID]*referencedMarkersMap),
		},
	}

	initialSequenceIndex := markers.HighestIndex() + 1
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := newReferencedMarkersMap()
		thresholdMap.Set(uint64(initialSequenceIndex), index)

		referencedMarkers.referencedMarkersInner.ReferencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// ReferencedMarkersFromBytes unmarshals ReferencedMarkers from a sequence of bytes.
func ReferencedMarkersFromBytes(parentReferencesBytes []byte) (referencedMarkers *ReferencedMarkers, consumedBytes int, err error) {
	referencedMarkers = new(ReferencedMarkers)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), parentReferencesBytes, referencedMarkers, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse ReferencedMarkers: %w", err)
		return
	}
	return
}

// Add adds new referenced Markers to the ReferencedMarkers.
func (r *ReferencedMarkers) Add(index Index, referencedMarkers *Markers) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := r.referencedMarkersInner.ReferencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = newReferencedMarkersMap()
			r.referencedMarkersInner.ReferencedIndexesBySequence[referencedSequenceID] = thresholdMap
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
	for sequenceID, thresholdMap := range r.referencedMarkersInner.ReferencedIndexesBySequence {
		if referencedIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencedMarkers.Set(sequenceID, referencedIndex)
		}
	}

	return
}

// Bytes returns a marshaled version of the ReferencingMarkers.
func (r *ReferencedMarkers) Bytes() []byte {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), r)
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the ReferencedMarkers.
func (r *ReferencedMarkers) String() (humanReadableReferencedMarkers string) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	indexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencedMarkersInner.ReferencedIndexesBySequence {
		thresholdMap.ForEach(func(node *thresholdmap.Element[uint64, Index]) bool {
			index := Index(node.Key())
			referencedIndex := node.Value()
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

// Markers flattens the collection and returns a normal Marker's collection by removing the rank information. The
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

// String returns a human-readable version of the markersByRank.
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

// region referencingMarkersMap /////////////////////////////////////////////////////////////////////////////////////////

type referencingMarkersMap struct {
	*thresholdmap.ThresholdMap[uint64, Index] `serix:"0"`
}

func newReferencingMarkersMap() *referencingMarkersMap {
	return &referencingMarkersMap{
		thresholdmap.New[uint64, Index](thresholdmap.UpperThresholdMode),
	}
}

// Encode returns a serialized byte slice of the object.
func (l *referencingMarkersMap) Encode() ([]byte, error) {
	return l.ThresholdMap.Encode()
}

// Decode deserializes bytes into a valid object.
func (l *referencingMarkersMap) Decode(b []byte) (bytesRead int, err error) {
	l.ThresholdMap = thresholdmap.New[uint64, Index](thresholdmap.UpperThresholdMode)
	return l.ThresholdMap.Decode(b)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region referencedMarkersMap /////////////////////////////////////////////////////////////////////////////////////////

type referencedMarkersMap struct {
	*thresholdmap.ThresholdMap[uint64, Index] `serix:"0"`
}

func newReferencedMarkersMap() *referencedMarkersMap {
	return &referencedMarkersMap{thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)}
}

// Encode returns a serialized byte slice of the object.
func (l *referencedMarkersMap) Encode() ([]byte, error) {
	return l.ThresholdMap.Encode()
}

// Decode deserializes bytes into a valid object.
func (l *referencedMarkersMap) Decode(b []byte) (bytesRead int, err error) {
	l.ThresholdMap = thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)
	return l.ThresholdMap.Decode(b)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

// Sequence represents a set of ever-increasing Indexes that are encapsulating a certain part of the DAG.
type Sequence struct {
	sequenceInner `serix:"0"`
}
type sequenceInner struct {
	id                               SequenceID
	ReferencedMarkers                *ReferencedMarkers  `serix:"0"`
	ReferencingMarkers               *ReferencingMarkers `serix:"1"`
	VerticesWithoutFutureMarker      uint64              `serix:"2"`
	LowestIndex                      Index               `serix:"3"`
	HighestIndex                     Index               `serix:"4"`
	verticesWithoutFutureMarkerMutex sync.RWMutex
	highestIndexMutex                sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewSequence creates a new Sequence from the given details.
func NewSequence(id SequenceID, referencedMarkers *Markers) *Sequence {
	initialIndex := referencedMarkers.HighestIndex() + 1

	if id == 0 {
		initialIndex--
	}

	return &Sequence{
		sequenceInner{
			id:                 id,
			ReferencedMarkers:  NewReferencedMarkers(referencedMarkers),
			ReferencingMarkers: NewReferencingMarkers(),
			LowestIndex:        initialIndex,
			HighestIndex:       initialIndex,
		},
	}
}

// FromObjectStorage creates a Sequence from sequences of key and bytes.
func (s *Sequence) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	sequence, err := s.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse Sequence from bytes: %w", err)
	}
	return sequence, err
}

// FromBytes unmarshals a Sequence from a sequence of bytes.
func (s *Sequence) FromBytes(data []byte) (sequence *Sequence, err error) {
	if sequence = s; sequence == nil {
		sequence = new(Sequence)
	}

	sequenceID := new(SequenceID)
	bytesRead, err := serix.DefaultAPI.Decode(context.Background(), data, sequenceID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Sequence.id: %w", err)
		return
	}

	_, err = serix.DefaultAPI.Decode(context.Background(), data[bytesRead:], sequence, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Sequence: %w", err)
		return
	}
	sequence.id = *sequenceID
	return

}

// ID returns the identifier of the Sequence.
func (s *Sequence) ID() SequenceID {
	return s.id
}

// ReferencedMarkers returns a collection of Markers that were referenced by the given Index.
func (s *Sequence) ReferencedMarkers(index Index) *Markers {
	return s.sequenceInner.ReferencedMarkers.Get(index)
}

// ReferencingMarkers returns a collection of Markers that reference the given Index.
func (s *Sequence) ReferencingMarkers(index Index) *Markers {
	return s.sequenceInner.ReferencingMarkers.Get(index)
}

// LowestIndex returns the Index of the very first Marker in the Sequence.
func (s *Sequence) LowestIndex() Index {
	return s.sequenceInner.LowestIndex
}

// HighestIndex returns the Index of the latest Marker in the Sequence.
func (s *Sequence) HighestIndex() Index {
	s.highestIndexMutex.RLock()
	defer s.highestIndexMutex.RUnlock()

	return s.sequenceInner.HighestIndex
}

// TryExtend tries to extend the Sequence with a new Index by checking if the referenced PastMarkers contain the last
// assigned Index of the Sequence. It returns the new Index, the remaining Markers pointing to other Sequences and a
// boolean flag that indicating if a new Index was assigned.
func (s *Sequence) TryExtend(referencedPastMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (index Index, remainingReferencedPastMarkers *Markers, extended bool) {
	s.highestIndexMutex.Lock()
	defer s.highestIndexMutex.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedPastMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to extend unreferenced Sequence")
	}

	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
	//  For example, (1,5) <-> (1,8) are valid subsequent structureDetails of sequence 1.
	if extended = referencedSequenceIndex == s.sequenceInner.HighestIndex && increaseIndexCallback(s.id, referencedSequenceIndex); extended {
		s.sequenceInner.HighestIndex = referencedPastMarkers.HighestIndex() + 1

		if referencedPastMarkers.Size() > 1 {
			remainingReferencedPastMarkers = referencedPastMarkers.Clone()
			remainingReferencedPastMarkers.Delete(s.id)

			s.sequenceInner.ReferencedMarkers.Add(s.sequenceInner.HighestIndex, remainingReferencedPastMarkers)
		}

		s.SetModified()
	}
	index = s.sequenceInner.HighestIndex

	return
}

// IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// Marker with the highest Index. It returns the new Index and a boolean flag that indicates if the value was
// increased.
func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
	s.highestIndexMutex.Lock()
	defer s.highestIndexMutex.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to increase Index of wrong Sequence")
	}

	if increased = referencedSequenceIndex >= s.sequenceInner.HighestIndex; increased {
		s.sequenceInner.HighestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.id)

			s.sequenceInner.ReferencedMarkers.Add(s.sequenceInner.HighestIndex, referencedMarkers)
		}

		s.SetModified()
	}
	index = s.sequenceInner.HighestIndex

	return
}

// AddReferencingMarker register a Marker that referenced the given Index of this Sequence.
func (s *Sequence) AddReferencingMarker(index Index, referencingMarker *Marker) {
	s.sequenceInner.ReferencingMarkers.Add(index, referencingMarker)

	s.SetModified()
}

// String returns a human-readable version of the Sequence.
func (s *Sequence) String() string {
	return stringify.Struct("Sequence",
		stringify.StructField("ID", s.ID()),
		stringify.StructField("LowestIndex", s.LowestIndex()),
		stringify.StructField("HighestIndex", s.HighestIndex()),
	)
}

// Bytes returns a marshaled version of the Sequence.
func (s *Sequence) Bytes() []byte {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *Sequence) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), s.id, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Sequence into a sequence of bytes that are used as the value part in the
// object storage.
func (s *Sequence) ObjectStorageValue() []byte {
	s.verticesWithoutFutureMarkerMutex.RLock()
	defer s.verticesWithoutFutureMarkerMutex.RUnlock()
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), s, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the type implements all required methods).
var _ objectstorage.StorableObject = new(Sequence)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceID ///////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceIDLength represents the amount of bytes of a marshaled SequenceID.
const SequenceIDLength = marshalutil.Uint64Size

// SequenceID is the type of the identifier of a Sequence.
type SequenceID uint64

// SequenceIDFromBytes unmarshals a SequenceID from a sequence of bytes.
func SequenceIDFromBytes(data []byte) (sequenceID SequenceID, consumedBytes int, err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, &sequenceID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse SequenceID: %w", err)
		return
	}
	return
}

// Bytes returns a marshaled version of the SequenceID.
func (a SequenceID) Bytes() (marshaledSequenceID []byte) {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the SequenceID.
func (a SequenceID) String() (humanReadableSequenceID string) {
	return "SequenceID(" + strconv.FormatUint(uint64(a), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceIDs //////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceIDs represents a collection of SequenceIDs.
type SequenceIDs map[SequenceID]types.Empty

// NewSequenceIDs creates a new collection of SequenceIDs.
func NewSequenceIDs(sequenceIDs ...SequenceID) (result SequenceIDs) {
	result = make(SequenceIDs)
	for _, sequenceID := range sequenceIDs {
		result[sequenceID] = types.Void
	}

	return
}

// String returns a human-readable version of the SequenceIDs.
func (s SequenceIDs) String() (humanReadableSequenceIDs string) {
	result := "SequenceIDs("
	firstItem := true
	for sequenceID := range s {
		if !firstItem {
			result += ", "
		}
		result += strconv.FormatUint(uint64(sequenceID), 10)

		firstItem = false
	}
	result += ")"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	Rank                     uint64   `serix:"0"`
	PastMarkerGap            uint64   `serix:"1"`
	IsPastMarker             bool     `serix:"2"`
	PastMarkers              *Markers `serix:"3"`
	FutureMarkers            *Markers `serix:"4"`
	futureMarkersUpdateMutex sync.Mutex
}

// StructureDetailsFromBytes unmarshals a StructureDetails from a sequence of bytes.
func StructureDetailsFromBytes(structureDetailBytes []byte) (marker *StructureDetails, consumedBytes int, err error) {
	marker = new(StructureDetails)
	consumedBytes, err = serix.DefaultAPI.Decode(context.Background(), structureDetailBytes, marker, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse StructureDetails: %w", err)
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
		PastMarkers:   m.PastMarkers.Clone(),
		FutureMarkers: m.FutureMarkers.Clone(),
	}
}

// Bytes returns a marshaled version of the StructureDetails.
func (m *StructureDetails) Bytes() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// String returns a human-readable version of the StructureDetails.
func (m *StructureDetails) String() (humanReadableStructureDetails string) {
	return stringify.Struct("StructureDetails",
		stringify.StructField("Rank", m.Rank),
		stringify.StructField("PastMarkerGap", m.PastMarkerGap),
		stringify.StructField("IsPastMarker", m.IsPastMarker),
		stringify.StructField("PastMarkers", m.PastMarkers),
		stringify.StructField("FutureMarkers", m.FutureMarkers),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
