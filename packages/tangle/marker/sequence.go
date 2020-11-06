package marker

import (
	"sort"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/cerrors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/mr-tron/base58"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/xerrors"
)

// region Sequence /////////////////////////////////////////////////////////////////////////////////////////////////////

type Sequence struct {
	id              SequenceID
	parentSequences SequenceIDs
	rank            uint64
	highestIndex    Index

	objectstorage.StorableObjectFlags
}

func NewSequence(id SequenceID, parentSequences SequenceIDs, rank uint64, highestIndex Index) *Sequence {
	return &Sequence{
		id:              id,
		parentSequences: parentSequences,
		rank:            rank,
		highestIndex:    highestIndex,
	}
}

func SequenceFromBytes(sequenceBytes []byte) (sequence *Sequence, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceBytes)
	if sequence, err = SequenceFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Sequence from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func SequenceFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequence *Sequence, err error) {
	sequence = &Sequence{}
	if sequence.id, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	if sequence.parentSequences, err = SequenceIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse parent SequenceIDs from MarshalUtil: %w", err)
		return
	}
	if sequence.rank, err = marshalUtil.ReadUint64(); err != nil {
		err = xerrors.Errorf("failed to parse rank (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if sequence.highestIndex, err = IndexFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse highest Index from MarshalUtil: %w", err)
		return
	}

	return
}

func SequenceFromObjectStorage(key []byte, data []byte) (sequence objectstorage.StorableObject, err error) {
	if sequence, _, err = SequenceFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Sequence from bytes: %w", err)
		return
	}

	return
}

func (s *Sequence) ID() SequenceID {
	return s.id
}

func (s *Sequence) ParentSequences() SequenceIDs {
	return s.parentSequences
}

func (s *Sequence) Rank() uint64 {
	return s.rank
}

func (s *Sequence) HighestIndex() Index {
	return s.highestIndex
}

func (s *Sequence) Bytes() []byte {
	return byteutils.ConcatBytes(s.ObjectStorageKey(), s.ObjectStorageValue())
}

func (s *Sequence) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (s *Sequence) ObjectStorageKey() []byte {
	return s.id.Bytes()
}

func (s *Sequence) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(s.parentSequences).
		WriteUint64(s.rank).
		Write(s.HighestIndex()).
		Bytes()
}

var _ objectstorage.StorableObject = &Sequence{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedSequence ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedSequence is a wrapper for the generic CachedObject returned by the objectstorage that
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

type SequenceID uint64

func SequenceIDFromBytes(sequenceIDBytes []byte) (sequenceID SequenceID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func SequenceIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceID SequenceID, err error) {
	untypedSequenceID, err := marshalUtil.ReadUint64()
	if err != nil {
		err = xerrors.Errorf("failed to parse SequenceID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceID = SequenceID(untypedSequenceID)

	return
}

func (a SequenceID) Bytes() []byte {
	return marshalutil.New(marshalutil.UINT16_SIZE).WriteUint64(uint64(a)).Bytes()
}

func (a SequenceID) String() string {
	return "SequenceID(" + strconv.FormatUint(uint64(a), 10) + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region SequenceIDs //////////////////////////////////////////////////////////////////////////////////////////////////

type SequenceIDs []SequenceID

func NewSequenceIDs(sequenceIDs ...SequenceID) (result SequenceIDs) {
	sort.Slice(sequenceIDs, func(i, j int) bool { return sequenceIDs[i] < sequenceIDs[j] })
	result = make(SequenceIDs, len(sequenceIDs))
	for i, sequenceID := range sequenceIDs {
		result[i] = sequenceID
	}

	return
}

func SequenceIDsFromBytes(sequenceIDBytes []byte) (sequenceIDs SequenceIDs, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(sequenceIDBytes)
	if sequenceIDs, err = SequenceIDsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse SequenceIDs from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func SequenceIDsFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (sequenceIDs SequenceIDs, err error) {
	sequenceIDsCount, err := marshalUtil.ReadUint32()
	if err != nil {
		err = xerrors.Errorf("failed to parse SequenceIDs count (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	sequenceIDs = make(SequenceIDs, sequenceIDsCount)
	for i := uint32(0); i < sequenceIDsCount; i++ {
		if sequenceIDs[i], err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse SequenceID from MarshalUtil: %w", err)
			return
		}
	}

	return
}

func (s SequenceIDs) AggregatedSequencesID() (aggregatedSequencesID AggregatedSequencesID) {
	marshalUtil := marshalutil.New(marshalutil.UINT64_SIZE * len(s))
	for sequenceID := range s {
		marshalUtil.WriteUint64(uint64(sequenceID))
	}
	aggregatedSequencesID = blake2b.Sum256(marshalUtil.Bytes())

	return
}

func (s SequenceIDs) Bytes() []byte {
	marshalUtil := marshalutil.New()
	marshalUtil.WriteUint32(uint32(len(s)))
	for _, sequenceID := range s {
		marshalUtil.Write(sequenceID)
	}

	return marshalUtil.Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedSequencesID ////////////////////////////////////////////////////////////////////////////////////////

const AggregatedSequencesIDLength = 32

type AggregatedSequencesID [AggregatedSequencesIDLength]byte

func AggregatedSequencesIDFromBytes(aggregatedSequencesIDBytes []byte) (aggregatedSequencesID AggregatedSequencesID, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(aggregatedSequencesIDBytes)
	if aggregatedSequencesID, err = AggregatedSequencesIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func AggregatedSequencesIDFromBase58EncodedString(base58String string) (aggregatedSequencesID AggregatedSequencesID, err error) {
	bytes, err := base58.Decode(base58String)
	if err != nil {
		err = xerrors.Errorf("error while decoding base58 encoded AggregatedSequencesID (%v): %w", err, cerrors.ErrBase58DecodeFailed)
		return
	}

	if aggregatedSequencesID, _, err = AggregatedSequencesIDFromBytes(bytes); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from bytes: %w", err)
		return
	}

	return
}

func AggregatedSequencesIDFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (aggregatedSequencesID AggregatedSequencesID, err error) {
	aggregatedSequencesIDBytes, err := marshalUtil.ReadBytes(AggregatedSequencesIDLength)
	if err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	copy(aggregatedSequencesID[:], aggregatedSequencesIDBytes)

	return
}

func (a AggregatedSequencesID) Bytes() []byte {
	return a[:]
}

func (a AggregatedSequencesID) Base58() string {
	return base58.Encode(a.Bytes())
}

func (a AggregatedSequencesID) String() string {
	return "AggregatedSequencesID(" + a.Base58() + ")"
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AggregatedSequencesIDMapping /////////////////////////////////////////////////////////////////////////////////

type AggregatedSequencesIDMapping struct {
	aggregatedSequencesID AggregatedSequencesID
	sequenceID            SequenceID

	objectstorage.StorableObjectFlags
}

func AggregatedSequencesIDMappingFromBytes(mappingBytes []byte) (mapping *AggregatedSequencesIDMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(mappingBytes)
	if mapping, err = AggregatedSequencesIDMappingFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesIDMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func AggregatedSequencesIDMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (mapping *AggregatedSequencesIDMapping, err error) {
	mapping = &AggregatedSequencesIDMapping{}
	if mapping.aggregatedSequencesID, err = AggregatedSequencesIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}
	if mapping.sequenceID, err = SequenceIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesID from MarshalUtil: %w", err)
		return
	}

	return
}

func AggregatedSequencesIDMappingFromObjectStorage(key []byte, data []byte) (mapping objectstorage.StorableObject, err error) {
	if mapping, _, err = AggregatedSequencesIDMappingFromBytes(data); err != nil {
		err = xerrors.Errorf("failed to parse AggregatedSequencesIDMapping from bytes: %w", err)
		return
	}

	return
}

func (a *AggregatedSequencesIDMapping) AggregatedSequencesID() AggregatedSequencesID {
	return a.aggregatedSequencesID
}

func (a *AggregatedSequencesIDMapping) SequenceID() SequenceID {
	return a.sequenceID
}

func (a *AggregatedSequencesIDMapping) Bytes() []byte {
	return byteutils.ConcatBytes(a.ObjectStorageKey(), a.ObjectStorageValue())
}

func (a *AggregatedSequencesIDMapping) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (a *AggregatedSequencesIDMapping) ObjectStorageKey() []byte {
	return a.aggregatedSequencesID.Bytes()
}

func (a *AggregatedSequencesIDMapping) ObjectStorageValue() []byte {
	return a.sequenceID.Bytes()
}

var _ objectstorage.StorableObject = &AggregatedSequencesIDMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedAggregatedSequencesIDMapping ///////////////////////////////////////////////////////////////////////////

// CachedAggregatedSequencesIDMapping is a wrapper for the generic CachedObject returned by the objectstorage that
// overrides the accessor methods with a type-casted one.
type CachedAggregatedSequencesIDMapping struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (c *CachedAggregatedSequencesIDMapping) Retain() *CachedAggregatedSequencesIDMapping {
	return &CachedAggregatedSequencesIDMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedAggregatedSequencesIDMapping) Unwrap() *AggregatedSequencesIDMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*AggregatedSequencesIDMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer. It automatically releases the
// object when the consumer finishes and returns true of there was at least one object that was consumed.
func (c *CachedAggregatedSequencesIDMapping) Consume(consumer func(aggregatedSequencesIDMapping *AggregatedSequencesIDMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*AggregatedSequencesIDMapping))
	}, forceRelease...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
