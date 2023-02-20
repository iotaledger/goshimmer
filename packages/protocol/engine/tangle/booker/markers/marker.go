package markers

import (
	"fmt"
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/ds/thresholdmap"
	"github.com/iotaledger/hive.go/serializer/v2/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
)

// region Index ////////////////////////////////////////////////////////////////////////////////////////////////////////

// Index represents the ever-increasing number of the Markers in a Sequence.
type Index uint64

// Length returns the amount of bytes of a serialized Index.
func (i Index) Length() int {
	return marshalutil.Uint64Size
}

// String returns a human-readable version of the Index.
func (i Index) String() (humanReadable string) {
	return "Index(" + strconv.FormatUint(uint64(i), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Marker represents a coordinate in a Sequence that is identified by an ever-increasing Index.
type Marker struct {
	sequenceID SequenceID
	index      Index
}

// NewMarker returns a new marker.
func NewMarker(sequenceID SequenceID, index Index) Marker {
	return Marker{
		sequenceID: sequenceID,
		index:      index,
	}
}

func (m Marker) SequenceID() SequenceID {
	return m.sequenceID
}

func (m Marker) Index() Index {
	return m.index
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	markers      map[SequenceID]Index
	highestIndex Index
	lowestIndex  Index
	mutex        sync.RWMutex
}

// NewMarkers creates a new collection of Markers.
func NewMarkers(markers ...Marker) (m *Markers) {
	m = &Markers{
		markers: make(map[SequenceID]Index),
	}

	for _, marker := range markers {
		m.Set(marker.SequenceID(), marker.Index())
	}

	return
}

// Marker type casts the Markers to a Marker if it contains only 1 element.
func (m *Markers) Marker() (marker Marker) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	switch len(m.markers) {
	case 0:
		panic("converting empty Markers into a single Marker is not supported")
	case 1:
		for sequenceID, index := range m.markers {
			return NewMarker(sequenceID, index)
		}

		return Marker{}
	default:
		panic("converting multiple Markers into a single Marker is not supported")
	}
}

// Get returns the Index of the Marker with the given Sequence and a flag that indicates if the Marker exists.
func (m *Markers) Get(sequenceID SequenceID) (index Index, exists bool) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	index, exists = m.markers[sequenceID]
	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and/or added.
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated, added bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if index > m.highestIndex {
		m.highestIndex = index
	}

	// if the sequence already exists in the set and the new index is higher than the old one then update
	if existingIndex, indexAlreadyStored := m.markers[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m.markers[sequenceID] = index

			// find new lowest index
			if existingIndex == m.lowestIndex {
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
	m.mutex.Lock()
	defer m.mutex.Unlock()

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
	if m == nil {
		return true
	}

	success = true
	for sequenceID, index := range m.Clone().markers {
		if success = iterator(sequenceID, index); !success {
			return
		}
	}

	return
}

// ForEachSorted calls the iterator for each of the contained Markers in increasing order. The iteration is aborted if
// the iterator returns false. The method returns false if the iteration was aborted.
func (m *Markers) ForEachSorted(iterator func(sequenceID SequenceID, index Index) bool) (success bool) {
	cloned := m.Clone()
	sequenceIDs := make([]SequenceID, 0, len(cloned.markers))
	for sequenceID := range cloned.markers {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}
	sort.Slice(sequenceIDs, func(i, j int) bool {
		return sequenceIDs[i] > sequenceIDs[j]
	})

	success = true
	for _, sequenceID := range sequenceIDs {
		if success = iterator(sequenceID, cloned.markers[sequenceID]); !success {
			return
		}
	}

	return
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
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.lowestIndex
}

// HighestIndex returns the highest Index of all Markers in the collection.
func (m *Markers) HighestIndex() (highestIndex Index) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return m.highestIndex
}

// Size returns the amount of Markers in the collection.
func (m *Markers) Size() (size int) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	return len(m.markers)
}

// Equals is a comparator for two Markers.
func (m *Markers) Equals(other *Markers) (equals bool) {
	if m.Size() != other.Size() {
		return false
	}

	for sequenceID, index := range m.markers {
		otherIndex, exists := other.Get(sequenceID)
		if !exists {
			return false
		}

		if otherIndex != index {
			return false
		}
	}

	return true
}

// Clone creates a deep copy of the Markers.
func (m *Markers) Clone() (cloned *Markers) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	cloned = NewMarkers()
	for sequenceID, index := range m.markers {
		cloned.Set(sequenceID, index)
	}

	return cloned
}

func (m *Markers) String() string {
	return fmt.Sprintf("Markers{%+v, highestIndex=%d, lowestIndex=%d}", m.markers, m.highestIndex, m.lowestIndex)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferencingMarkers ///////////////////////////////////////////////////////////////////////////////////////////

// ReferencingMarkers is a data structure that allows to denote which Markers of child Sequences in the Sequence DAG
// reference a given Marker in a Sequence.
type ReferencingMarkers struct {
	referencingIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index] `serix:"0,lengthPrefixType=uint32"`
	sync.RWMutex
}

// NewReferencingMarkers is the constructor for the ReferencingMarkers.
func NewReferencingMarkers() (referencingMarkers *ReferencingMarkers) {
	return &ReferencingMarkers{
		referencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index]),
	}
}

// Add adds a new referencing Marker to the ReferencingMarkers.
func (r *ReferencingMarkers) Add(index Index, referencingMarker Marker) {
	r.Lock()
	defer r.Unlock()

	thresholdMap, thresholdMapExists := r.referencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = thresholdmap.New[uint64, Index](thresholdmap.UpperThresholdMode)
		r.referencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(index), referencingMarker.Index())
}

// Get returns the Markers of child Sequences that reference the given Index.
func (r *ReferencingMarkers) Get(index Index) (referencingMarkers *Markers) {
	r.RLock()
	defer r.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
		if referencingIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencingMarkers.Set(sequenceID, referencingIndex)
		}
	}

	return
}

// GetSequenceIDs returns the SequenceIDs of child Sequences.
func (r *ReferencingMarkers) GetSequenceIDs() (referencingSequenceIDs SequenceIDs) {
	r.RLock()
	defer r.RUnlock()

	referencingSequenceIDs = NewSequenceIDs()
	for sequenceID := range r.referencingIndexesBySequence {
		referencingSequenceIDs.Add(sequenceID)
	}

	return
}

// String returns a human-readable version of the ReferencingMarkers.
func (r *ReferencingMarkers) String() (humanReadableReferencingMarkers string) {
	r.RLock()
	defer r.RUnlock()

	indexes := make([]Index, 0)
	referencingMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencingIndexesBySequence {
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
	referencingMarkers := stringify.NewStructBuilder("ReferencingMarkers")
	for _, index := range indexes {
		thresholdEnd := strconv.FormatUint(uint64(index), 10)

		if thresholdStart == thresholdEnd {
			referencingMarkers.AddField(stringify.NewStructField("Index("+thresholdStart+")", referencingMarkersByReferencingIndex[index]))
		} else {
			referencingMarkers.AddField(stringify.NewStructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencingMarkersByReferencingIndex[index]))
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
	referencedIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index] `serix:"0,lengthPrefixType=uint32"`
	sync.RWMutex
}

// NewReferencedMarkers is the constructor for the ReferencedMarkers.
func NewReferencedMarkers(markers *Markers) (r *ReferencedMarkers) {
	r = &ReferencedMarkers{
		referencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index]),
	}
	initialSequenceIndex := markers.HighestIndex() + 1
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)
		thresholdMap.Set(uint64(initialSequenceIndex), index)

		r.referencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// Add adds new referenced Markers to the ReferencedMarkers.
func (r *ReferencedMarkers) Add(index Index, referencedMarkers *Markers) {
	r.Lock()
	defer r.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := r.referencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)
			r.referencedIndexesBySequence[referencedSequenceID] = thresholdMap
		}

		thresholdMap.Set(uint64(index), referencedIndex)

		return true
	})
}

func (r *ReferencedMarkers) Delete(id SequenceID) {
	r.Lock()
	defer r.Unlock()
	delete(r.referencedIndexesBySequence, id)
}

// Get returns the Markers of parent Sequences that were referenced by the given Index.
func (r *ReferencedMarkers) Get(index Index) (referencedMarkers *Markers) {
	r.RLock()
	defer r.RUnlock()

	referencedMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
		if referencedIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencedMarkers.Set(sequenceID, referencedIndex)
		}
	}

	return
}

// String returns a human-readable version of the ReferencedMarkers.
func (r *ReferencedMarkers) String() (humanReadableReferencedMarkers string) {
	r.RLock()
	defer r.RUnlock()

	indexes := make([]Index, 0)
	referencedMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.referencedIndexesBySequence {
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

	referencedMarkers := stringify.NewStructBuilder("ReferencedMarkers")
	for i, index := range indexes {
		thresholdStart := strconv.FormatUint(uint64(index), 10)
		thresholdEnd := "INF"
		if len(indexes) > i+1 {
			thresholdEnd = strconv.FormatUint(uint64(indexes[i+1])-1, 10)
		}

		if thresholdStart == thresholdEnd {
			referencedMarkers.AddField(stringify.NewStructField("Index("+thresholdStart+")", referencedMarkersByReferencingIndex[index]))
		} else {
			referencedMarkers.AddField(stringify.NewStructField("Index("+thresholdStart+" ... "+thresholdEnd+")", referencedMarkersByReferencingIndex[index]))
		}
	}

	return referencedMarkers.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
