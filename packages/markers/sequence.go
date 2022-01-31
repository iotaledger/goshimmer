package markers

import (
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
)

// maxVerticesWithoutFutureMarker defines the amount of vertices in the DAG are allowed to have no future marker before
// we spawn a new Sequence for the same SequenceAlias.
const maxVerticesWithoutFutureMarker = 3000

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

// Sequence represents a set of ever increasing Indexes that are encapsulating a certain part of the DAG.
type Sequence struct {
	id                               SequenceID
	referencedMarkers                *ReferencedMarkers
	referencingMarkers               *ReferencingMarkers
	rank                             uint64
	lowestIndex                      Index
	highestIndex                     Index
	newSequenceTrigger               uint64
	maxPastMarkerGap                 uint64
	verticesWithoutFutureMarker      uint64
	verticesWithoutFutureMarkerMutex sync.RWMutex
	highestIndexMutex                sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewSequence creates a new Sequence from the given details.
func NewSequence(id SequenceID, referencedMarkers *Markers, rank uint64) *Sequence {
	initialIndex := referencedMarkers.HighestIndex() + 1

	if id == 0 {
		initialIndex--
	}

	return &Sequence{
		id:                 id,
		referencedMarkers:  NewReferencedMarkers(referencedMarkers),
		referencingMarkers: NewReferencingMarkers(),
		rank:               rank,
		lowestIndex:        initialIndex,
		highestIndex:       initialIndex,
	}
}

// SequenceFromBytes unmarshals a Sequence from a sequence of bytes.
func SequenceFromBytes(sequenceBytes []byte) (sequence *Sequence, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if sequence, err = SequenceFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Sequence from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceFromMarshalUtil unmarshals a Sequence using a MarshalUtil (for easier unmarshaling).
func SequenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequence *Sequence, err error) {
	sequence = &Sequence{}
	if sequence.id, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	if sequence.referencedMarkers, err = ReferencedMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ReferencedMarkers from MarshalUtil: %w", err)
		return
	}
	if sequence.referencingMarkers, err = ReferencingMarkersFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse ReferencingMarkers from MarshalUtil: %w", err)
		return
	}
	if sequence.rank, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse rank (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.newSequenceTrigger, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse newSequenceTrigger (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.maxPastMarkerGap, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse maxPastMarkerGap (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.verticesWithoutFutureMarker, err = marshalUtil.ReadUint64(); err != nil {
		err = errors.Errorf("failed to parse verticesWithoutFutureMarker (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.lowestIndex, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse lowest Index from MarshalUtil: %w", err)
		return
	}
	if sequence.highestIndex, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse highest Index from MarshalUtil: %w", err)
		return
	}

	return
}

// SequenceFromObjectStorage restores an Sequence that was stored in the object storage.
func SequenceFromObjectStorage(key, data []byte) (sequence objectstorage.StorableObject, err error) {
	if sequence, _, err = SequenceFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse Sequence from bytes: %w", err)
		return
	}

	return
}

// ID returns the identifier of the Sequence.
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

// Rank returns the rank of the Sequence (maximum distance from the root of the Sequence DAG).
func (s *Sequence) Rank() uint64 {
	return s.rank
}

// LowestIndex returns the Index of the very first Marker in the Sequence.
func (s *Sequence) LowestIndex() Index {
	return s.lowestIndex
}

// HighestIndex returns the Index of the latest Marker in the Sequence.
func (s *Sequence) HighestIndex() Index {
	s.highestIndexMutex.RLock()
	defer s.highestIndexMutex.RUnlock()

	return s.highestIndex
}

// IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// Marker with the currently highest Index. It returns the new Index and a boolean flag that indicates if the value was
// increased.
func (s *Sequence) ExtendSequence(referencedMarkers *Markers, increaseIndexCallback IncreaseIndexCallback) (index Index, increased bool) {
	s.highestIndexMutex.Lock()
	defer s.highestIndexMutex.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to extend unreferenced Sequence")
	}

	// TODO: this is a quick'n'dirty solution and should be revisited.
	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
	//  For example, (1,5) <-> (1,8) are valid subsequent markers of sequence 1.
	if increased = referencedSequenceIndex == s.highestIndex && increaseIndexCallback(s.id, referencedSequenceIndex); increased {
		s.highestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.id)

			s.referencedMarkers.Add(s.highestIndex, referencedMarkers)
		}

		s.SetModified()
	}
	index = s.highestIndex

	return
}

// IncreaseHighestIndex increases the highest Index of the Sequence if the referencedMarkers directly reference the
// Marker with the currently highest Index. It returns the new Index and a boolean flag that indicates if the value was
// increased.
func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
	s.highestIndexMutex.Lock()
	defer s.highestIndexMutex.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to increase Index of wrong Sequence")
	}

	// TODO: this is a quick'n'dirty solution and should be revisited.
	//  referencedSequenceIndex >= s.highestIndex allows gaps in a marker sequence to exist.
	//  For example, (1,5) <-> (1,8) are valid subsequent markers of sequence 1.
	if increased = referencedSequenceIndex >= s.highestIndex; increased {
		s.highestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.id)

			s.referencedMarkers.Add(s.highestIndex, referencedMarkers)
		}

		s.SetModified()
	}
	index = s.highestIndex

	return
}

// AddReferencingMarker register a Marker that referenced the given Index of this Sequence.
func (s *Sequence) AddReferencingMarker(index Index, referencingMarker *Marker) {
	s.referencingMarkers.Add(index, referencingMarker)

	s.SetModified()
}

// String returns a human readable version of the Sequence.
func (s *Sequence) String() string {
	return stringify.Struct("Sequence",
		stringify.StructField("ID", s.ID()),
		stringify.StructField("Rank", s.Rank()),
		stringify.StructField("LowestIndex", s.LowestIndex()),
		stringify.StructField("HighestIndex", s.HighestIndex()),
	)
}

// Bytes returns a marshaled version of the Sequence.
func (s *Sequence) Bytes() []byte {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

// Update is required to match the StorableObject interface but updates of the object are disabled.
func (s *Sequence) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (s *Sequence) ObjectStorageKey() []byte {
	return s.id.Bytes()
}

// ObjectStorageValue marshals the Sequence into a sequence of bytes. The ID is not serialized here as it is only used as
// a key in the object storage.
func (s *Sequence) ObjectStorageValue() []byte {
	s.verticesWithoutFutureMarkerMutex.RLock()
	defer s.verticesWithoutFutureMarkerMutex.RUnlock()

	return marshalutil.New().
		Write(s.referencedMarkers).
		Write(s.referencingMarkers).
		WriteUint64(s.rank).
		WriteUint64(s.newSequenceTrigger).
		WriteUint64(s.maxPastMarkerGap).
		WriteUint64(s.verticesWithoutFutureMarker).
		Write(s.lowestIndex).
		Write(s.HighestIndex()).
		Bytes()
}

func (s *Sequence) increaseVerticesWithoutFutureMarker() {
	s.verticesWithoutFutureMarkerMutex.Lock()
	defer s.verticesWithoutFutureMarkerMutex.Unlock()

	s.verticesWithoutFutureMarker++

	s.SetModified()
}

func (s *Sequence) decreaseVerticesWithoutFutureMarker() {
	s.verticesWithoutFutureMarkerMutex.Lock()
	defer s.verticesWithoutFutureMarkerMutex.Unlock()

	s.verticesWithoutFutureMarker--

	s.SetModified()
}

func (s *Sequence) newSequenceRequired(pastMarkerGap uint64) (newSequenceRequired bool) {
	s.verticesWithoutFutureMarkerMutex.Lock()
	defer s.verticesWithoutFutureMarkerMutex.Unlock()

	s.SetModified()

	// decrease the maxPastMarkerGap threshold with every processed message so it ultimately goes back to a healthy
	// level after the triggering
	if s.maxPastMarkerGap > 0 {
		s.maxPastMarkerGap--
	}

	if pastMarkerGap > s.maxPastMarkerGap {
		s.maxPastMarkerGap = pastMarkerGap
	}

	if s.verticesWithoutFutureMarker < maxVerticesWithoutFutureMarker {
		return false
	}

	if s.newSequenceTrigger == 0 {
		s.newSequenceTrigger = s.maxPastMarkerGap
		return false
	}

	if pastMarkerGap < s.newSequenceTrigger {
		return false
	}

	s.newSequenceTrigger = 0

	return true
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &Sequence{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedSequence ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedSequence is a wrapper for the generic CachedObject returned by the object storage that
// overrides the accessor methods with a type-casted one.
type CachedSequence struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (c *CachedSequence) Retain() *CachedSequence {
	return &CachedSequence{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedSequence) Unwrap() *Sequence {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Sequence)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer. It automatically releases the
// object when the consumer finishes and returns true of there was at least one object that was consumed.
func (c *CachedSequence) Consume(consumer func(sequence *Sequence), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Sequence))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceID ///////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceIDLength represents the amount of bytes of a marshaled SequenceID.
const SequenceIDLength = marshalutil.Uint64Size

// SequenceID is the type of the identifier of a Sequence.
type SequenceID uint64

// SequenceIDFromBytes unmarshals a SequenceID from a sequence of bytes.
func SequenceIDFromBytes(sequenceIDBytes []byte) (sequenceID SequenceID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceIDFromMarshalUtil unmarshals a SequenceIDs using a MarshalUtil (for easier unmarshaling).
func SequenceIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceID SequenceID, err error) {
	untypedSequenceID, err := marshalUtil.ReadUint64()
	if err != nil {
		err = errors.Errorf("failed to parse SequenceID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceID = SequenceID(untypedSequenceID)

	return
}

// Bytes returns a marshaled version of the SequenceID.
func (a SequenceID) Bytes() (marshaledSequenceID []byte) {
	return marshalutil.New(marshalutil.Uint64Size).WriteUint64(uint64(a)).Bytes()
}

// String returns a human readable version of the SequenceID.
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

// SequenceIDsFromBytes unmarshals a collection of SequenceIDs from a sequence of bytes.
func SequenceIDsFromBytes(sequenceIDBytes []byte) (sequenceIDs SequenceIDs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceIDs, err = SequenceIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SequenceIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceIDsFromMarshalUtil unmarshals a collection of SequenceIDs using a MarshalUtil (for easier unmarshaling).
func SequenceIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceIDs SequenceIDs, err error) {
	sequenceIDsCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = errors.Errorf("failed to parse SequenceIDs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceIDs = make(SequenceIDs, sequenceIDsCount)
	for i := uint32(0); i < sequenceIDsCount; i++ {
		sequenceID, sequenceIDErr := SequenceIDFromMarshalUtil(marshalUtil)
		if sequenceIDErr != nil {
			err = errors.Errorf("failed to parse SequenceID from MarshalUtil: %w", sequenceIDErr)
			return
		}
		sequenceIDs[sequenceID] = types.Void
	}

	return
}

// Bytes returns a marshaled version of the SequenceIDs.
func (s SequenceIDs) Bytes() (marshaledSequenceIDs []byte) {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(s)))
	for sequenceID := range s {
		marshalUtil.Write(sequenceID)
	}

	return marshalUtil.Bytes()
}

// String returns a human readable version of the SequenceIDs.
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
