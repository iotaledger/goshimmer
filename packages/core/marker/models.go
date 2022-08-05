package marker

import (
	"context"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/marshalutil"
	"github.com/iotaledger/hive.go/core/serix"
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
	SequenceID SequenceID
	Index      Index
}

// NewMarker returns a new marker.
func NewMarker(sequenceID SequenceID, index Index) Marker {
	return Marker{
		SequenceID: sequenceID,
		Index:      index,
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Markers //////////////////////////////////////////////////////////////////////////////////////////////////////

// Markers represents a collection of Markers that can contain exactly one Index per SequenceID.
type Markers struct {
	markers map[SequenceID]Index
	// HighestIndex Index
	// LowestIndex  Index
	sync.RWMutex
}

// NewMarkers creates a new collection of Markers.
func NewMarkers(markers ...Marker) (m *Markers) {
	m = &Markers{
		markers: make(map[SequenceID]Index),
	}

	for _, marker := range markers {
		m.Set(marker.SequenceID, marker.Index)
	}

	return
}

// Set adds a new Marker to the collection and updates the Index of an existing entry if it is higher than a possible
// previously stored one. The method returns two boolean flags that indicate if an entry was updated and/or added.
func (m *Markers) Set(sequenceID SequenceID, index Index) (updated, added bool) {
	m.Lock()
	defer m.Unlock()

	m.markers[sequenceID] = index

	return true, true
}

// Size returns the amount of Markers in the collection.
func (m *Markers) Size() (size int) {
	m.RLock()
	defer m.RUnlock()

	return len(m.markers)
}

// Clone creates a deep copy of the Markers.
func (m *Markers) Clone() (cloned *Markers) {
	m.RLock()
	defer m.RUnlock()

	cloned = NewMarkers()
	for sequenceID, index := range m.markers {
		cloned.Set(sequenceID, index)
	}

	return cloned
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// // region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////
//
// // Sequence represents a set of ever-increasing Indexes that are encapsulating a certain part of the DAG.
// type Sequence struct {
// 	model.Storable[SequenceID, Sequence, *Sequence, sequenceModel] `serix:"0"`
// }
//
// type sequenceModel struct {
// 	ReferencedMarkers  *ReferencedMarkers  `serix:"0"`
// 	ReferencingMarkers *ReferencingMarkers `serix:"1"`
// 	LowestIndex        Index               `serix:"2"`
// 	HighestIndex       Index               `serix:"3"`
// }
//
// // NewSequence creates a new Sequence from the given details.
// func NewSequence(id SequenceID, referencedMarkers *Markers) (new *Sequence) {
// 	initialIndex := referencedMarkers.HighestIndex() + 1
// 	if id == 0 {
// 		initialIndex--
// 	}
//
// 	new = model.NewStorable[SequenceID, Sequence](&sequenceModel{
// 		ReferencedMarkers:  NewReferencedMarkers(referencedMarkers),
// 		ReferencingMarkers: NewReferencingMarkers(),
// 		LowestIndex:        initialIndex,
// 		HighestIndex:       initialIndex,
// 	})
// 	new.SetID(id)
//
// 	return new
// }
//
// // ReferencedMarkers returns a collection of Markers that were referenced by the given Index.
// func (s *Sequence) ReferencedMarkers(index Index) *Markers {
// 	return s.M.ReferencedMarkers.Get(index)
// }
//
// // ReferencingMarkers returns a collection of Markers that reference the given Index.
// func (s *Sequence) ReferencingMarkers(index Index) *Markers {
// 	return s.M.ReferencingMarkers.Get(index)
// }
//
// // LowestIndex returns the Index of the very first Marker in the Sequence.
// func (s *Sequence) LowestIndex() Index {
// 	s.RLock()
// 	defer s.RUnlock()
//
// 	return s.M.LowestIndex
// }
//
// // HighestIndex returns the Index of the latest Marker in the Sequence.
// func (s *Sequence) HighestIndex() Index {
// 	s.RLock()
// 	defer s.RUnlock()
//
// 	return s.M.HighestIndex
// }
//
// // TryExtend tries to extend the Sequence with a new Index by checking if the referenced PastMarkers contain the last
// // assigned Index of the Sequence. It returns the new Index, the remaining Markers pointing to other Sequences and a
// // boolean flag that indicating if a new Index was assigned.
// func (s *Sequence) TryExtend(referencedPastMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (index Index, remainingReferencedPastMarkers *Markers, extended bool) {
// 	s.Lock()
// 	defer s.Unlock()
//
// 	referencedSequenceIndex, referencedSequenceIndexExists := referencedPastMarkers.Get(s.ID())
// 	if !referencedSequenceIndexExists {
// 		panic("tried to extend unreferenced Sequence")
// 	}
//
// 	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
// 	//  For example, (1,5) <-> (1,8) are valid subsequent structureDetails of sequence 1.
// 	if extended = referencedSequenceIndex == s.M.HighestIndex && increaseIndexCallback(s.ID(), referencedSequenceIndex); extended {
// 		s.M.HighestIndex = referencedPastMarkers.HighestIndex() + 1
//
// 		if referencedPastMarkers.Size() > 1 {
// 			remainingReferencedPastMarkers = referencedPastMarkers.Clone()
// 			remainingReferencedPastMarkers.Delete(s.ID())
//
// 			s.M.ReferencedMarkers.Add(s.M.HighestIndex, remainingReferencedPastMarkers)
// 		}
//
// 		s.SetModified()
// 	}
// 	index = s.M.HighestIndex
//
// 	return
// }
//
// // IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// // Marker with the highest Index. It returns the new Index and a boolean flag that indicates if the value was
// // increased.
// func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
// 	s.Lock()
// 	defer s.Unlock()
//
// 	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.ID())
// 	if !referencedSequenceIndexExists {
// 		panic("tried to increase Index of wrong Sequence")
// 	}
//
// 	if increased = referencedSequenceIndex >= s.M.HighestIndex; increased {
// 		s.M.HighestIndex = referencedMarkers.HighestIndex() + 1
//
// 		if referencedMarkers.Size() > 1 {
// 			referencedMarkers.Delete(s.ID())
//
// 			s.M.ReferencedMarkers.Add(s.M.HighestIndex, referencedMarkers)
// 		}
//
// 		s.SetModified()
// 	}
// 	index = s.M.HighestIndex
//
// 	return
// }
//
// // AddReferencingMarker register a Marker that referenced the given Index of this Sequence.
// func (s *Sequence) AddReferencingMarker(index Index, referencingMarker Marker) {
// 	s.M.ReferencingMarkers.Add(index, referencingMarker)
//
// 	s.SetModified()
// }
//
// // endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

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
type SequenceIDs *set.AdvancedSet[SequenceID]

// NewSequenceIDs creates a new collection of SequenceIDs.
func NewSequenceIDs(sequenceIDs ...SequenceID) (result SequenceIDs) {
	result = set.NewAdvancedSet(sequenceIDs...)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region StructureDetails /////////////////////////////////////////////////////////////////////////////////////////////

// StructureDetails represents a container for the complete Marker related information of a node in a DAG that are used
// to interact with the public API of this package.
type StructureDetails struct {
	rank          uint64
	pastMarkerGap uint64
	isPastMarker  bool
	pastMarkers   *Markers

	sync.RWMutex
}

// NewStructureDetails creates an empty StructureDetails object.
func NewStructureDetails() (s *StructureDetails) {
	return &StructureDetails{
		pastMarkers: NewMarkers(),
	}
}

func (s *StructureDetails) Rank() (rank uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.rank
}

func (s *StructureDetails) SetRank(rank uint64) {
	s.Lock()
	defer s.Unlock()

	s.rank = rank
}

func (s *StructureDetails) PastMarkerGap() (pastMarkerGap uint64) {
	s.RLock()
	defer s.RUnlock()

	return s.pastMarkerGap
}

func (s *StructureDetails) SetPastMarkerGap(pastMarkerGap uint64) {
	s.Lock()
	defer s.Unlock()

	s.pastMarkerGap = pastMarkerGap
}

func (s *StructureDetails) IsPastMarker() (isPastMarker bool) {
	s.RLock()
	defer s.RUnlock()

	return s.isPastMarker
}

func (s *StructureDetails) SetIsPastMarker(isPastMarker bool) {
	s.Lock()
	defer s.Unlock()

	s.isPastMarker = isPastMarker
}

func (s *StructureDetails) PastMarkers() (pastMarkers *Markers) {
	s.RLock()
	defer s.RUnlock()

	return s.pastMarkers
}

func (s *StructureDetails) SetPastMarkers(pastMarkers *Markers) {
	s.Lock()
	defer s.Unlock()

	s.pastMarkers = pastMarkers
}

// Clone creates a deep copy of the StructureDetails.
func (s *StructureDetails) Clone() (clone *StructureDetails) {
	return &StructureDetails{
		rank:          s.Rank(),
		pastMarkerGap: s.PastMarkerGap(),
		isPastMarker:  s.IsPastMarker(),
		pastMarkers:   s.PastMarkers().Clone(),
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
