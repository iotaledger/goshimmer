package markers

import (
	"context"
	"sort"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/model"
	"github.com/iotaledger/hive.go/core/generics/thresholdmap"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/serix"
	"github.com/iotaledger/hive.go/core/stringify"
	"github.com/iotaledger/hive.go/core/types"
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

// region IncreaseIndexCallback ////////////////////////////////////////////////////////////////////////////////////////

// IncreaseIndexCallback is the type of the callback function that is used to determine if a new Index is supposed to be
// assigned in a given Sequence.
type IncreaseIndexCallback func(sequenceID SequenceID, currentHighestIndex Index) bool

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Marker ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Marker represents a coordinate in a Sequence that is identified by an ever-increasing Index.
type Marker struct {
	model.Immutable[Marker, *Marker, markerModel] `serix:"0"`
}

// markerModel contains the data of a Marker.
type markerModel struct {
	SequenceID SequenceID `serix:"0"`
	Index      Index      `serix:"1"`
}

// NewMarker returns a new marker.
func NewMarker(sequenceID SequenceID, index Index) Marker {
	return *model.NewImmutable[Marker](&markerModel{
		SequenceID: sequenceID,
		Index:      index,
	})
}

// SequenceID returns the identifier of the Sequence of the Marker.
func (m Marker) SequenceID() (sequenceID SequenceID) {
	return m.M.SequenceID
}

// Index returns the coordinate of the Marker in a Sequence.
func (m Marker) Index() (index Index) {
	return m.M.Index
}

// Bytes returns a serialized version of the Marker.
func (m Marker) Bytes() (serialized []byte) {
	return lo.PanicOnErr(m.Immutable.Bytes())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	model.Mutable[Markers, *Markers, markersModel] `serix:"0"`
}

type markersModel struct {
	Markers      map[SequenceID]Index `serix:"0,lengthPrefixType=uint32"`
	HighestIndex Index                `serix:"1"`
	LowestIndex  Index                `serix:"2"`
}

// NewMarkers creates a new collection of Markers.
func NewMarkers(markers ...Marker) (new *Markers) {
	new = model.NewMutable[Markers](&markersModel{
		Markers: make(map[SequenceID]Index),
	})

	for _, marker := range markers {
		new.Set(marker.SequenceID(), marker.Index())
	}

	return
}

// Marker type casts the Markers to a Marker if it contains only 1 element.
func (m *Markers) Marker() (marker Marker) {
	m.RLock()
	defer m.RUnlock()

	switch len(m.M.Markers) {
	case 0:
		panic("converting empty Markers into a single Marker is not supported")
	case 1:
		for sequenceID, index := range m.M.Markers {
			return NewMarker(sequenceID, index)
		}

		return Marker{}
	default:
		panic("converting multiple Markers into a single Marker is not supported")
	}
}

// Get returns the Index of the Marker with the given Sequence and a flag that indicates if the Marker exists.
func (m *Markers) Get(sequenceID SequenceID) (index Index, exists bool) {
	m.RLock()
	defer m.RUnlock()

	index, exists = m.M.Markers[sequenceID]
	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and/or added.
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated, added bool) {
	m.Lock()
	defer m.Unlock()

	if index > m.M.HighestIndex {
		m.M.HighestIndex = index
	}

	// if the sequence already exists in the set and the new index is higher than the old one then update
	if existingIndex, indexAlreadyStored := m.M.Markers[sequenceID]; indexAlreadyStored {
		if updated = index > existingIndex; updated {
			m.M.Markers[sequenceID] = index

			// find new lowest index
			if existingIndex == m.M.LowestIndex {
				m.M.LowestIndex = 0
				for _, scannedIndex := range m.M.Markers {
					if scannedIndex < m.M.LowestIndex || m.M.LowestIndex == 0 {
						m.M.LowestIndex = scannedIndex
					}
				}
			}
		}

		return
	}

	// if this is a new sequence update lowestIndex
	if index < m.M.LowestIndex || m.M.LowestIndex == 0 {
		m.M.LowestIndex = index
	}

	m.M.Markers[sequenceID] = index

	return true, true
}

// Delete removes the Marker with the given SequenceID from the collection and returns a boolean flag that indicates if
// the element existed.
func (m *Markers) Delete(sequenceID SequenceID) (existed bool) {
	m.Lock()
	defer m.Unlock()

	existingIndex, existed := m.M.Markers[sequenceID]
	delete(m.M.Markers, sequenceID)
	if existed {
		lowestIndexDeleted := existingIndex == m.M.LowestIndex
		if lowestIndexDeleted {
			m.M.LowestIndex = 0
		}
		highestIndexDeleted := existingIndex == m.M.HighestIndex
		if highestIndexDeleted {
			m.M.HighestIndex = 0
		}

		if lowestIndexDeleted || highestIndexDeleted {
			for _, scannedIndex := range m.M.Markers {
				if scannedIndex < m.M.LowestIndex || m.M.LowestIndex == 0 {
					m.M.LowestIndex = scannedIndex
				}
				if scannedIndex > m.M.HighestIndex {
					m.M.HighestIndex = scannedIndex
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
	for sequenceID, index := range m.Clone().M.Markers {
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
	sequenceIDs := make([]SequenceID, 0, len(cloned.M.Markers))
	for sequenceID := range cloned.M.Markers {
		sequenceIDs = append(sequenceIDs, sequenceID)
	}
	sort.Slice(sequenceIDs, func(i, j int) bool {
		return sequenceIDs[i] > sequenceIDs[j]
	})

	success = true
	for _, sequenceID := range sequenceIDs {
		if success = iterator(sequenceID, cloned.M.Markers[sequenceID]); !success {
			return
		}
	}

	return
}

// Size returns the amount of Markers in the collection.
func (m *Markers) Size() (size int) {
	m.RLock()
	defer m.RUnlock()

	return len(m.M.Markers)
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
	m.RLock()
	defer m.RUnlock()

	return m.M.LowestIndex
}

// HighestIndex returns the highest Index of all Markers in the collection.
func (m *Markers) HighestIndex() (highestIndex Index) {
	m.RLock()
	defer m.RUnlock()

	return m.M.HighestIndex
}

// Clone creates a deep copy of the Markers.
func (m *Markers) Clone() (cloned *Markers) {
	m.RLock()
	defer m.RUnlock()

	cloned = NewMarkers()
	for sequenceID, index := range m.M.Markers {
		cloned.Set(sequenceID, index)
	}

	return cloned
}

// Equals is a comparator for two Markers.
func (m *Markers) Equals(other *Markers) (equals bool) {
	if m.Size() != other.Size() {
		return false
	}

	for sequenceID, index := range m.M.Markers {
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

// Bytes returns a marshaled version of the Markers.
func (m *Markers) Bytes() []byte {
	m.RLock()
	defer m.RUnlock()

	return lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), m, serix.WithValidation()))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ReferencingMarkers ///////////////////////////////////////////////////////////////////////////////////////////

// ReferencingMarkers is a data structure that allows to denote which Markers of child Sequences in the Sequence DAG
// reference a given Marker in a Sequence.
type ReferencingMarkers struct {
	model.Mutable[ReferencingMarkers, *ReferencingMarkers, referencingMarkersModel] `serix:"0"`
}

type referencingMarkersModel struct {
	ReferencingIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index] `serix:"0,lengthPrefixType=uint32"`
}

// NewReferencingMarkers is the constructor for the ReferencingMarkers.
func NewReferencingMarkers() (referencingMarkers *ReferencingMarkers) {
	return model.NewMutable[ReferencingMarkers](&referencingMarkersModel{
		ReferencingIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index]),
	})
}

// Add adds a new referencing Marker to the ReferencingMarkers.
func (r *ReferencingMarkers) Add(index Index, referencingMarker Marker) {
	r.Lock()
	defer r.Unlock()

	thresholdMap, thresholdMapExists := r.M.ReferencingIndexesBySequence[referencingMarker.SequenceID()]
	if !thresholdMapExists {
		thresholdMap = thresholdmap.New[uint64, Index](thresholdmap.UpperThresholdMode)
		r.M.ReferencingIndexesBySequence[referencingMarker.SequenceID()] = thresholdMap
	}

	thresholdMap.Set(uint64(index), referencingMarker.Index())
}

// Get returns the Markers of child Sequences that reference the given Index.
func (r *ReferencingMarkers) Get(index Index) (referencingMarkers *Markers) {
	r.RLock()
	defer r.RUnlock()

	referencingMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.M.ReferencingIndexesBySequence {
		if referencingIndex, exists := thresholdMap.Get(uint64(index)); exists {
			referencingMarkers.Set(sequenceID, referencingIndex)
		}
	}

	return
}

// String returns a human-readable version of the ReferencingMarkers.
func (r *ReferencingMarkers) String() (humanReadableReferencingMarkers string) {
	r.RLock()
	defer r.RUnlock()

	indexes := make([]Index, 0)
	referencingMarkersByReferencingIndex := make(map[Index]*Markers)
	for sequenceID, thresholdMap := range r.M.ReferencingIndexesBySequence {
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
	model.Mutable[ReferencedMarkers, *ReferencedMarkers, referencedMarkersModel] `serix:"0"`
}
type referencedMarkersModel struct {
	ReferencedIndexesBySequence map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index] `serix:"0,lengthPrefixType=uint32"`
}

// NewReferencedMarkers is the constructor for the ReferencedMarkers.
func NewReferencedMarkers(markers *Markers) (new *ReferencedMarkers) {
	new = model.NewMutable[ReferencedMarkers](&referencedMarkersModel{
		ReferencedIndexesBySequence: make(map[SequenceID]*thresholdmap.ThresholdMap[uint64, Index]),
	})

	initialSequenceIndex := markers.HighestIndex() + 1
	markers.ForEach(func(sequenceID SequenceID, index Index) bool {
		thresholdMap := thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)
		thresholdMap.Set(uint64(initialSequenceIndex), index)

		new.M.ReferencedIndexesBySequence[sequenceID] = thresholdMap

		return true
	})

	return
}

// Add adds new referenced Markers to the ReferencedMarkers.
func (r *ReferencedMarkers) Add(index Index, referencedMarkers *Markers) {
	r.Lock()
	defer r.Unlock()

	referencedMarkers.ForEach(func(referencedSequenceID SequenceID, referencedIndex Index) bool {
		thresholdMap, exists := r.M.ReferencedIndexesBySequence[referencedSequenceID]
		if !exists {
			thresholdMap = thresholdmap.New[uint64, Index](thresholdmap.LowerThresholdMode)
			r.M.ReferencedIndexesBySequence[referencedSequenceID] = thresholdMap
		}

		thresholdMap.Set(uint64(index), referencedIndex)

		return true
	})
}

// Get returns the Markers of parent Sequences that were referenced by the given Index.
func (r *ReferencedMarkers) Get(index Index) (referencedMarkers *Markers) {
	r.RLock()
	defer r.RUnlock()

	referencedMarkers = NewMarkers()
	for sequenceID, thresholdMap := range r.M.ReferencedIndexesBySequence {
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
	for sequenceID, thresholdMap := range r.M.ReferencedIndexesBySequence {
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

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

// Sequence represents a set of ever-increasing Indexes that are encapsulating a certain part of the DAG.
type Sequence struct {
	model.Storable[SequenceID, Sequence, *Sequence, sequenceModel] `serix:"0"`
}

type sequenceModel struct {
	ReferencedMarkers  *ReferencedMarkers  `serix:"0"`
	ReferencingMarkers *ReferencingMarkers `serix:"1"`
	LowestIndex        Index               `serix:"2"`
	HighestIndex       Index               `serix:"3"`
}

// NewSequence creates a new Sequence from the given details.
func NewSequence(id SequenceID, referencedMarkers *Markers) (new *Sequence) {
	initialIndex := referencedMarkers.HighestIndex() + 1
	if id == 0 {
		initialIndex--
	}

	new = model.NewStorable[SequenceID, Sequence](&sequenceModel{
		ReferencedMarkers:  NewReferencedMarkers(referencedMarkers),
		ReferencingMarkers: NewReferencingMarkers(),
		LowestIndex:        initialIndex,
		HighestIndex:       initialIndex,
	})
	new.SetID(id)

	return new
}

// ReferencedMarkers returns a collection of Markers that were referenced by the given Index.
func (s *Sequence) ReferencedMarkers(index Index) *Markers {
	return s.M.ReferencedMarkers.Get(index)
}

// ReferencingMarkers returns a collection of Markers that reference the given Index.
func (s *Sequence) ReferencingMarkers(index Index) *Markers {
	return s.M.ReferencingMarkers.Get(index)
}

// LowestIndex returns the Index of the very first Marker in the Sequence.
func (s *Sequence) LowestIndex() Index {
	s.RLock()
	defer s.RUnlock()

	return s.M.LowestIndex
}

// HighestIndex returns the Index of the latest Marker in the Sequence.
func (s *Sequence) HighestIndex() Index {
	s.RLock()
	defer s.RUnlock()

	return s.M.HighestIndex
}

// TryExtend tries to extend the Sequence with a new Index by checking if the referenced PastMarkers contain the last
// assigned Index of the Sequence. It returns the new Index, the remaining Markers pointing to other Sequences and a
// boolean flag that indicating if a new Index was assigned.
func (s *Sequence) TryExtend(referencedPastMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (index Index, remainingReferencedPastMarkers *Markers, extended bool) {
	s.Lock()
	defer s.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedPastMarkers.Get(s.ID())
	if !referencedSequenceIndexExists {
		panic("tried to extend unreferenced Sequence")
	}

	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
	//  For example, (1,5) <-> (1,8) are valid subsequent structureDetails of sequence 1.
	if extended = referencedSequenceIndex == s.M.HighestIndex && increaseIndexCallback(s.ID(), referencedSequenceIndex); extended {
		s.M.HighestIndex = referencedPastMarkers.HighestIndex() + 1

		if referencedPastMarkers.Size() > 1 {
			remainingReferencedPastMarkers = referencedPastMarkers.Clone()
			remainingReferencedPastMarkers.Delete(s.ID())

			s.M.ReferencedMarkers.Add(s.M.HighestIndex, remainingReferencedPastMarkers)
		}

		s.SetModified()
	}
	index = s.M.HighestIndex

	return
}

// IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// Marker with the highest Index. It returns the new Index and a boolean flag that indicates if the value was
// increased.
func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
	s.Lock()
	defer s.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.ID())
	if !referencedSequenceIndexExists {
		panic("tried to increase Index of wrong Sequence")
	}

	if increased = referencedSequenceIndex >= s.M.HighestIndex; increased {
		s.M.HighestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.ID())

			s.M.ReferencedMarkers.Add(s.M.HighestIndex, referencedMarkers)
		}

		s.SetModified()
	}
	index = s.M.HighestIndex

	return
}

// AddReferencingMarker register a Marker that referenced the given Index of this Sequence.
func (s *Sequence) AddReferencingMarker(index Index, referencingMarker Marker) {
	s.M.ReferencingMarkers.Add(index, referencingMarker)

	s.SetModified()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceID ///////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceID is the type of the identifier of a Sequence.
type SequenceID uint64

// FromBytes unmarshals a SequenceID from a sequence of bytes.
func (s *SequenceID) FromBytes(data []byte) (err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, s, serix.WithValidation())
	if err != nil {
		return errors.Errorf("failed to parse SequenceID: %w", err)
	}

	return nil
}

// Length returns the length of a serialized SequenceID.
func (s SequenceID) Length() int {
	return marshalutil.Uint64Size
}

// Bytes returns a marshaled version of the SequenceID.
func (s SequenceID) Bytes() (marshaledSequenceID []byte) {
	return lo.PanicOnErr(serix.DefaultAPI.Encode(context.Background(), s, serix.WithValidation()))
}

// String returns a human-readable version of the SequenceID.
func (s SequenceID) String() (humanReadableSequenceID string) {
	return "SequenceID(" + strconv.FormatUint(uint64(s), 10) + ")"
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
	model.Mutable[StructureDetails, *StructureDetails, structureDetailsModel] `serix:"0"`
}

type structureDetailsModel struct {
	Rank          uint64   `serix:"0"`
	PastMarkerGap uint64   `serix:"1"`
	IsPastMarker  bool     `serix:"2"`
	PastMarkers   *Markers `serix:"3"`
}

// NewStructureDetails creates an empty StructureDetails object.
func NewStructureDetails() (newStructureDetails *StructureDetails) {
	return model.NewMutable[StructureDetails](&structureDetailsModel{
		PastMarkers: NewMarkers(),
	})
}

func (m *StructureDetails) Rank() (rank uint64) {
	m.RLock()
	defer m.RUnlock()

	return m.M.Rank
}

func (m *StructureDetails) SetRank(rank uint64) {
	m.Lock()
	defer m.Unlock()

	m.M.Rank = rank
}

func (m *StructureDetails) PastMarkerGap() (pastMarkerGap uint64) {
	m.RLock()
	defer m.RUnlock()

	return m.M.PastMarkerGap
}

func (m *StructureDetails) SetPastMarkerGap(pastMarkerGap uint64) {
	m.Lock()
	defer m.Unlock()

	m.M.PastMarkerGap = pastMarkerGap
}

func (m *StructureDetails) IsPastMarker() (isPastMarker bool) {
	m.RLock()
	defer m.RUnlock()

	return m.M.IsPastMarker
}

func (m *StructureDetails) SetIsPastMarker(isPastMarker bool) {
	m.Lock()
	defer m.Unlock()

	m.M.IsPastMarker = isPastMarker
}

func (m *StructureDetails) PastMarkers() (pastMarkers *Markers) {
	m.RLock()
	defer m.RUnlock()

	return m.M.PastMarkers
}

func (m *StructureDetails) SetPastMarkers(pastMarkers *Markers) {
	m.Lock()
	defer m.Unlock()

	m.M.PastMarkers = pastMarkers
}

// Clone creates a deep copy of the StructureDetails.
func (m *StructureDetails) Clone() (clone *StructureDetails) {
	return model.NewMutable[StructureDetails](&structureDetailsModel{
		Rank:          m.Rank(),
		PastMarkerGap: m.PastMarkerGap(),
		IsPastMarker:  m.IsPastMarker(),
		PastMarkers:   m.PastMarkers().Clone(),
	})
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
