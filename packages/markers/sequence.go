package markers

import (
	"sort"
	"strconv"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

// Sequence represents a set of ever increasing Indexes that are encapsulating a certain part of the DAG.
type Sequence struct {
	id                SequenceID
	parentReferences  *ParentReferences
	rank              uint64
	lowestIndex       Index
	highestIndex      Index
	highestIndexMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewSequence creates a new Sequence from the given details.
func NewSequence(id SequenceID, referencedMarkers *Markers, rank uint64) *Sequence {
	initialIndex := referencedMarkers.HighestIndex() + 1

	return &Sequence{
		id:               id,
		parentReferences: NewParentReferences(referencedMarkers),
		rank:             rank,
		lowestIndex:      initialIndex,
		highestIndex:     initialIndex,
	}
}

// SequenceFromBytes unmarshals a Sequence from a sequence of bytes.
func SequenceFromBytes(sequenceBytes []byte) (sequence *Sequence, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if sequence, err = SequenceFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Sequence from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceFromMarshalUtil unmarshals a Sequence using a MarshalUtil (for easier unmarshaling).
func SequenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequence *Sequence, err error) {
	sequence = &Sequence{}
	if sequence.id, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	if sequence.parentReferences, err = ParentReferencesFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse ParentReferences from MarshalUtil: %w", err)
		return
	}
	if sequence.rank, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse rank (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.lowestIndex, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse lowest Index from MarshalUtil: %w", err)
		return
	}
	if sequence.highestIndex, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse highest Index from MarshalUtil: %w", err)
		return
	}

	return
}

// SequenceFromObjectStorage restores an Sequence that was stored in the object storage.
func SequenceFromObjectStorage(key []byte, data []byte) (sequence objectstorage.StorableObject, err error) {
	if sequence, _, err = SequenceFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Sequence from bytes: %w", err)
		return
	}

	return
}

// ID returns the identifier of the Sequence.
func (s *Sequence) ID() SequenceID {
	return s.id
}

// ParentSequences returns the SequenceIDs of the parent Sequences in the Sequence DAG.
func (s *Sequence) ParentSequences() SequenceIDs {
	return s.parentReferences.SequenceIDs()
}

// HighestReferencedParentMarkers returns a collection of Markers that were referenced by the given Index.
func (s *Sequence) HighestReferencedParentMarkers(index Index) *Markers {
	return s.parentReferences.HighestReferencedMarkers(index)
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
func (s *Sequence) IncreaseHighestIndex(referencedMarkers *Markers) (index Index, increased bool) {
	s.highestIndexMutex.Lock()
	defer s.highestIndexMutex.Unlock()

	referencedSequenceIndex, referencedSequenceIndexExists := referencedMarkers.Get(s.id)
	if !referencedSequenceIndexExists {
		panic("tried to increase Index of wrong Sequence")
	}

	if increased = referencedSequenceIndex == s.highestIndex; increased {
		s.highestIndex = referencedMarkers.HighestIndex() + 1

		if referencedMarkers.Size() > 1 {
			referencedMarkers.Delete(s.id)

			s.parentReferences.AddReferences(referencedMarkers, s.highestIndex)
		}
	}
	index = s.highestIndex

	return
}

// Bytes returns a marshaled version of the Sequence.
func (s *Sequence) Bytes() []byte {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

// Update is required to match the StorableObject interface but updates of the object are disabled.
func (s *Sequence) Update(other objectstorage.StorableObject) {
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
	return marshalutil.New().
		Write(s.parentReferences).
		WriteUint64(s.rank).
		Write(s.lowestIndex).
		Write(s.HighestIndex()).
		Bytes()
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

// SequenceID is the type of the identifier of a Sequence.
type SequenceID uint64

// SequenceIDFromBytes unmarshals a SequenceID from a sequence of bytes.
func SequenceIDFromBytes(sequenceIDBytes []byte) (sequenceID SequenceID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceIDFromMarshalUtil unmarshals a SequenceIDs using a MarshalUtil (for easier unmarshaling).
func SequenceIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceID SequenceID, err error) {
	untypedSequenceID, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse SequenceID (%v): %w", err, cerrors.ErrParseBytesFailed)
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
		err = xerrors.Errorf("failed to parse SequenceIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceIDsFromMarshalUtil unmarshals a collection of SequenceIDs using a MarshalUtil (for easier unmarshaling).
func SequenceIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceIDs SequenceIDs, err error) {
	sequenceIDsCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse SequenceIDs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceIDs = make(SequenceIDs, sequenceIDsCount)
	for i := uint32(0); i < sequenceIDsCount; i++ {
		sequenceID, sequenceIDErr := SequenceIDFromMarshalUtil(marshalUtil)
		if sequenceIDErr != nil {
			err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", sequenceIDErr)
			return
		}
		sequenceIDs[sequenceID] = types.Void
	}

	return
}

// Alias returns the SequenceAlias of the SequenceIDs. The SequenceAlias is used to address the numerical Sequences
// in the object storage.
func (s SequenceIDs) Alias() (aggregatedSequencesID SequenceAlias) {
	sequenceIDsSlice := make([]SequenceID, 0, len(s))
	for sequenceID := range s {
		sequenceIDsSlice = append(sequenceIDsSlice, sequenceID)
	}
	sort.Slice(sequenceIDsSlice, func(i, j int) bool { return sequenceIDsSlice[i] < sequenceIDsSlice[j] })

	marshalUtil := marshalutil.New(marshalutil.Uint64Size * len(s))
	for _, sequenceID := range sequenceIDsSlice {
		marshalUtil.WriteUint64(uint64(sequenceID))
	}
	aggregatedSequencesID = blake2b.Sum256(marshalUtil.Bytes())

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
	for sequenceID := range s {
		if len(result) != 12 {
			result += ", "
		}
		result += strconv.FormatUint(uint64(sequenceID), 10)
	}
	result += ")"

	return result
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceAlias ////////////////////////////////////////////////////////////////////////////////////////////////

// SequenceAliasLength contains the amount of bytes that a marshaled version of the SequenceAlias contains.
const SequenceAliasLength = 32

// SequenceAlias represents an alternative identifier for a Sequence that is used to look up the SequenceID.
type SequenceAlias [SequenceAliasLength]byte

// NewSequenceAlias creates a new custom identifier from a sequence of bytes.
func NewSequenceAlias(bytes []byte) SequenceAlias {
	return blake2b.Sum256(bytes)
}

// SequenceAliasFromBytes unmarshals a SequenceAlias from a sequence of bytes.
func SequenceAliasFromBytes(aggregatedSequencesIDBytes []byte) (aggregatedSequencesID SequenceAlias, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(aggregatedSequencesIDBytes)
	if aggregatedSequencesID, err = SequenceAliasFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Alias from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceAliasFromBase58 creates a SequenceAlias from a base58 encoded string.
func SequenceAliasFromBase58(base58String string) (aggregatedSequencesID SequenceAlias, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded Alias (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if aggregatedSequencesID, _, err = SequenceAliasFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse Alias from bytes: %w", err)
		return
	}

	return
}

// SequenceAliasFromMarshalUtil unmarshals a SequenceAlias using a MarshalUtil (for easier unmarshaling).
func SequenceAliasFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (aggregatedSequencesID SequenceAlias, err error) {
	aggregatedSequencesIDBytes, err := marshalUtil.ReadBytes(SequenceAliasLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse Alias (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(aggregatedSequencesID[:], aggregatedSequencesIDBytes)

	return
}

// Merge generates a new unique SequenceAlias from the combination of two SequenceAliases.
func (a SequenceAlias) Merge(alias SequenceAlias) (mergedSequenceAlias SequenceAlias) {
	byteutils.XORBytes(mergedSequenceAlias[:], a[:], alias[:])

	return
}

// Bytes returns a marshaled version of the SequenceAlias.
func (a SequenceAlias) Bytes() (marshaledSequenceAlias []byte) {
	return a[:]
}

// Base58 returns a base58 encoded version of the SequenceAlias.
func (a SequenceAlias) Base58() (base58EncodedSequenceAlias string) {
	return base58.Encode(a.Bytes())
}

// String returns a human readable version of the SequenceAlias.
func (a SequenceAlias) String() (humanReadableSequenceAlias string) {
	return "Alias(" + a.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceAliasMapping /////////////////////////////////////////////////////////////////////////////////////////

// SequenceAliasMapping represents the mapping between a SequenceAlias and its SequenceID.
type SequenceAliasMapping struct {
	sequenceAlias SequenceAlias
	sequenceID    SequenceID

	objectstorage.StorableObjectFlags
}

// SequenceAliasMappingFromBytes unmarshals a SequenceAliasMapping from a sequence of bytes.
func SequenceAliasMappingFromBytes(mappingBytes []byte) (mapping *SequenceAliasMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(mappingBytes)
	if mapping, err = SequenceAliasMappingFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceAliasMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SequenceAliasMappingFromMarshalUtil unmarshals a SequenceAliasMapping using a MarshalUtil (for easier unmarshaling).
func SequenceAliasMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (mapping *SequenceAliasMapping, err error) {
	mapping = &SequenceAliasMapping{}
	if mapping.sequenceAlias, err = SequenceAliasFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Alias from MarshalUtil: %w", err)
		return
	}
	if mapping.sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}

	return
}

// SequenceAliasMappingFromObjectStorage restores a SequenceAlias that was stored in the object storage.
func SequenceAliasMappingFromObjectStorage(key []byte, data []byte) (mapping objectstorage.StorableObject, err error) {
	if mapping, _, err = SequenceAliasMappingFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse SequenceAliasMapping from bytes: %w", err)
		return
	}

	return
}

// SequenceAlias returns the SequenceAlias of SequenceAliasMapping.
func (a *SequenceAliasMapping) SequenceAlias() (sequenceAlias SequenceAlias) {
	return a.sequenceAlias
}

// SequenceID returns the SequenceID of the SequenceAliasMapping.
func (a *SequenceAliasMapping) SequenceID() (sequenceID SequenceID) {
	return a.sequenceID
}

// Bytes returns a marshaled version of the SequenceAliasMapping.
func (a *SequenceAliasMapping) Bytes() (marshaledSequenceAliasMapping []byte) {
	return byteutils.ConcatBytes(a.ObjectStorageKey(), a.ObjectStorageValue())
}

// Update is required to match the StorableObject interface but updates of the object are disabled.
func (a *SequenceAliasMapping) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *SequenceAliasMapping) ObjectStorageKey() (objectStorageKey []byte) {
	return a.sequenceAlias.Bytes()
}

// ObjectStorageValue marshals the Transaction into a sequence of bytes. The ID is not serialized here as it is only
// used as a key in the object storage.
func (a *SequenceAliasMapping) ObjectStorageValue() (objectStorageValue []byte) {
	return a.sequenceID.Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &SequenceAliasMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedSequenceAliasMapping ///////////////////////////////////////////////////////////////////////////////////

// CachedSequenceAliasMapping is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedSequenceAliasMapping struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (c *CachedSequenceAliasMapping) Retain() *CachedSequenceAliasMapping {
	return &CachedSequenceAliasMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedSequenceAliasMapping) Unwrap() *SequenceAliasMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*SequenceAliasMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer. It automatically releases the
// object when the consumer finishes and returns true of there was at least one object that was consumed.
func (c *CachedSequenceAliasMapping) Consume(consumer func(aggregatedSequencesIDMapping *SequenceAliasMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*SequenceAliasMapping))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
