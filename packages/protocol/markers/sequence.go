package markers

import (
	"context"
	"strconv"
	"sync"

	"github.com/pkg/errors"

	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/serializer/v2/marshalutil"
	"github.com/iotaledger/hive.go/serializer/v2/serix"
)

// region SequenceID ///////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceID is the type of the identifier of a Sequence.
type SequenceID uint64

// FromBytes unmarshals a SequenceID from a sequence of bytes.
func (s *SequenceID) FromBytes(data []byte) (err error) {
	_, err = serix.DefaultAPI.Decode(context.Background(), data, s, serix.WithValidation())
	if err != nil {
		return errors.Wrap(err, "failed to parse SequenceID")
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
type SequenceIDs = *advancedset.AdvancedSet[SequenceID]

// NewSequenceIDs creates a new collection of SequenceIDs.
func NewSequenceIDs(sequenceIDs ...SequenceID) (result SequenceIDs) {
	result = advancedset.New(sequenceIDs...)

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

// Sequence represents a set of ever-increasing Indexes that are encapsulating a certain part of the DAG.
type Sequence struct {
	id                 SequenceID
	referencedMarkers  *ReferencedMarkers
	referencingMarkers *ReferencingMarkers
	lowestIndex        Index
	highestIndex       Index

	sync.RWMutex
}

// NewSequence creates a new Sequence from the given details.
func NewSequence(id SequenceID, referencedMarkers *Markers) (s *Sequence) {
	initialIndex := referencedMarkers.HighestIndex() + 1

	// if no referenced markers passed, it means that the block attached to one of the solid entry points and lowestIndex=Index(0) signifies that.
	if referencedMarkers.Size() == 0 {
		initialIndex--
	}

	s = &Sequence{
		id:                 id,
		referencedMarkers:  NewReferencedMarkers(referencedMarkers),
		referencingMarkers: NewReferencingMarkers(),
		lowestIndex:        initialIndex,
		highestIndex:       initialIndex,
	}

	return s
}

func (s *Sequence) ID() SequenceID {
	return s.id
}

// ReferencedMarkers returns a collection of Markers that were referenced by the given Index.
func (s *Sequence) ReferencedMarkers(index Index) *Markers {
	return s.referencedMarkers.Get(index)
}

// ReferencingMarkers returns a collection of Markers that reference the given Index.
func (s *Sequence) ReferencingMarkers(index Index) *Markers {
	return s.referencingMarkers.Get(index)
}

// ReferencingSequences returns a collection of SequenceIDs that reference the Sequence.
func (s *Sequence) ReferencingSequences() SequenceIDs {
	return s.referencingMarkers.GetSequenceIDs()
}

// LowestIndex returns the Index of the very first Marker in the Sequence.
func (s *Sequence) LowestIndex() Index {
	s.RLock()
	defer s.RUnlock()

	return s.lowestIndex
}

// HighestIndex returns the Index of the latest Marker in the Sequence.
func (s *Sequence) HighestIndex() Index {
	s.RLock()
	defer s.RUnlock()

	return s.highestIndex
}

// TryExtend tries to extend the Sequence with a new Index by checking if the referenced PastMarkers contain the last
// assigned Index of the Sequence. It returns the new Index, the remaining Markers pointing to other Sequences and a
// boolean flag that indicating if a new Index was assigned.
func (s *Sequence) TryExtend(referencedPastMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (index Index, remainingReferencedPastMarkers *Markers, extended bool) {
	s.Lock()
	defer s.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedPastMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to extend unreferenced Sequence")
	}

	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
	//  For example, (1,5) <-> (1,8) are valid subsequent structureDetails of sequence 1.
	if extended = referencedSequenceIndex == s.highestIndex && increaseIndexCallback(s.ID(), referencedSequenceIndex); extended {
		s.highestIndex = referencedPastMarkers.HighestIndex() + 1

		if referencedPastMarkers.Size() > 1 {
			remainingReferencedPastMarkers = referencedPastMarkers.Clone()
			remainingReferencedPastMarkers.Delete(s.id)

			s.referencedMarkers.Add(s.highestIndex, remainingReferencedPastMarkers)
		}
	}
	index = s.highestIndex

	return
}

// IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// Marker with the highest Index. It returns the new Index and a boolean flag that indicates if the value was
// increased.
func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
	s.Lock()
	defer s.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to increase Index of wrong Sequence")
	}

	if increased = referencedSequenceIndex >= s.highestIndex; increased {
		s.highestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.id)

			s.referencedMarkers.Add(s.highestIndex, referencedMarkers)
		}
	}
	index = s.highestIndex

	return
}

// AddReferencingMarker register a Marker that referenced the given Index of this Sequence.
func (s *Sequence) AddReferencingMarker(index Index, referencingMarker Marker) {
	s.referencingMarkers.Add(index, referencingMarker)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region IncreaseIndexCallback ////////////////////////////////////////////////////////////////////////////////////////

// IncreaseIndexCallback is the type of the callback function that is used to determine if a new Index is supposed to be
// assigned in a given Sequence.
type IncreaseIndexCallback func(sequenceID SequenceID, currentHighestIndex Index) bool

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
