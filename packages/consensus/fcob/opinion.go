package fcob

import (
	"fmt"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// LevelOfKnowledge defines the Level Of Knowledge type.
type LevelOfKnowledge uint8

// The different levels of knowledge.
const (
	// Pending implies that the opinion is not formed yet.
	Pending LevelOfKnowledge = iota

	// One implies that voting is required.
	One

	// Two implies that we have locally finalized our opinion but we can still reply to eventual queries.
	Two

	// Three implies that we have locally finalized our opinion and we do not reply to eventual queries.
	Three
)

// String returns a human readable version of LevelOfKnowledge.
func (l LevelOfKnowledge) String() string {
	switch l {
	case Pending:
		return "LevelOfKnowledge(Pending)"
	case One:
		return "LevelOfKnowledge(One)"
	case Two:
		return "LevelOfKnowledge(Two)"
	case Three:
		return "LevelOfKnowledge(Three)"
	default:
		return fmt.Sprintf("LevelOfKnowledge(%X)", uint8(l))
	}
}

// region TransactionOpinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionEssence contains the essence of an opinion (timestamp, liked, level of knowledge).
type OpinionEssence struct {
	// timestamp is the arrrival/solidification time.
	timestamp time.Time

	// liked is the opinion value.
	liked bool

	// levelOfKnowledge is the degree of certainty of the associated opinion.
	levelOfKnowledge LevelOfKnowledge
}

// TransactionOpinion defines the FCoB opinion about a transaction.
type TransactionOpinion struct {
	// the transactionID associated to this opinion.
	transactionID ledgerstate.TransactionID

	// the essence associated to this opinion.
	OpinionEssence

	timestampMutex        sync.RWMutex
	likedMutex            sync.RWMutex
	levelOfKnowledgeMutex sync.RWMutex

	fcobTime1     time.Time
	fcobTime2     time.Time
	fcobTimeMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// Timestamp returns the opinion's timestamp.
func (o OpinionEssence) Timestamp() time.Time {
	return o.timestamp
}

// Liked returns the opinion's liked.
func (o OpinionEssence) Liked() bool {
	return o.liked
}

// LevelOfKnowledge returns the opinion's LevelOfKnowledge.
func (o OpinionEssence) LevelOfKnowledge() LevelOfKnowledge {
	return o.levelOfKnowledge
}

func (o OpinionEssence) String() string {
	return stringify.Struct("OpinionEssence",
		stringify.StructField("Timestamp", o.timestamp),
		stringify.StructField("Liked", o.liked),
		stringify.StructField("LoK", o.levelOfKnowledge),
	)
}

// Timestamp returns the opinion's timestamp.
func (o *TransactionOpinion) Timestamp() time.Time {
	o.timestampMutex.RLock()
	defer o.timestampMutex.RUnlock()
	return o.timestamp
}

// SetTimestamp sets the opinion's timestamp.
func (o *TransactionOpinion) SetTimestamp(t time.Time) {
	o.timestampMutex.Lock()
	defer o.timestampMutex.Unlock()
	o.timestamp = t
	o.SetModified(true)
}

// Liked returns the opinion's liked.
func (o *TransactionOpinion) Liked() bool {
	o.likedMutex.RLock()
	defer o.likedMutex.RUnlock()
	return o.liked
}

// SetLiked sets the opinion's liked.
func (o *TransactionOpinion) SetLiked(l bool) {
	o.likedMutex.Lock()
	defer o.likedMutex.Unlock()
	o.liked = l
	o.SetModified(true)
}

// LevelOfKnowledge returns the opinion's LevelOfKnowledge.
func (o *TransactionOpinion) LevelOfKnowledge() LevelOfKnowledge {
	o.levelOfKnowledgeMutex.RLock()
	defer o.levelOfKnowledgeMutex.RUnlock()
	return o.levelOfKnowledge
}

// SetLevelOfKnowledge sets the opinion's LevelOfKnowledge.
func (o *TransactionOpinion) SetLevelOfKnowledge(lok LevelOfKnowledge) {
	o.levelOfKnowledgeMutex.Lock()
	defer o.levelOfKnowledgeMutex.Unlock()
	o.levelOfKnowledge = lok
	o.SetModified(true)
}

// SetFCOBTime1 sets the opinion's LikedThreshold execution time.
func (o *TransactionOpinion) SetFCOBTime1(t time.Time) {
	o.fcobTimeMutex.Lock()
	defer o.fcobTimeMutex.Unlock()
	o.fcobTime1 = t
	o.SetModified()
}

// FCOBTime1 returns the opinion's LikedThreshold execution time.
func (o *TransactionOpinion) FCOBTime1() time.Time {
	o.fcobTimeMutex.RLock()
	defer o.fcobTimeMutex.RUnlock()
	return o.fcobTime1
}

// SetFCOBTime2 sets the opinion's LocallyFinalizedThreshold execution time.
func (o *TransactionOpinion) SetFCOBTime2(t time.Time) {
	o.fcobTimeMutex.Lock()
	defer o.fcobTimeMutex.Unlock()
	o.fcobTime2 = t
	o.SetModified()
}

// FCOBTime2 returns the opinion's LocallyFinalizedThreshold execution time.
func (o *TransactionOpinion) FCOBTime2() time.Time {
	o.fcobTimeMutex.RLock()
	defer o.fcobTimeMutex.RUnlock()
	return o.fcobTime2
}

// Bytes marshals the TransactionOpinion into a sequence of bytes.
func (o *TransactionOpinion) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human readable version of the TransactionOpinion.
func (o *TransactionOpinion) String() string {
	return stringify.Struct("TransactionOpinion",
		stringify.StructField("transactionID", o.transactionID),
		stringify.StructField("timestamp", o.Timestamp),
		stringify.StructField("liked", o.Liked),
		stringify.StructField("LevelOfKnowledge", o.LevelOfKnowledge),
		stringify.StructField("fcobTime1", o.FCOBTime1()),
		stringify.StructField("fcobTime2", o.FCOBTime2()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *TransactionOpinion) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *TransactionOpinion) ObjectStorageKey() []byte {
	return o.transactionID.Bytes()
}

// ObjectStorageValue marshals the TransactionOpinion into a sequence of bytes. The ID is not serialized here as it is
// only used as a key in the ObjectStorage.
func (o *TransactionOpinion) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(o.Timestamp()).
		WriteBool(o.Liked()).
		WriteUint8(uint8(o.LevelOfKnowledge())).
		WriteTime(o.FCOBTime1()).
		WriteTime(o.FCOBTime2()).
		Bytes()
}

// OpinionFromMarshalUtil parses an opinion from the given marshal util.
func OpinionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *TransactionOpinion, err error) {
	// parse information
	result = &TransactionOpinion{}
	if result.timestamp, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse opinion timestamp from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if result.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag of the opinion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	levelOfKnowledgeUint8, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse level of knowledge of the opinion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.levelOfKnowledge = LevelOfKnowledge(levelOfKnowledgeUint8)

	if result.fcobTime1, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse opinion fcob time 1 from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	if result.fcobTime2, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse opinion fcob time 2 from MarshalUtil (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TransactionOpinionFromObjectStorage restores an TransactionOpinion from the ObjectStorage.
func TransactionOpinionFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	// parse the opinion
	opinion, err := OpinionFromMarshalUtil(marshalutil.New(data))
	if err != nil {
		err = xerrors.Errorf("failed to parse opinion from object Storage (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	// parse the TransactionID from they key
	id, err := ledgerstate.TransactionIDFromMarshalUtil(marshalutil.New(key))
	if err != nil {
		err = xerrors.Errorf("failed to parse transaction ID from object Storage (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	opinion.transactionID = id

	// assign result
	result = opinion

	return
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &TransactionOpinion{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTransactionOpinion ////////////////////////////////////////////////////////////////////////////////////////////////

// CachedTransactionOpinion is a wrapper for the generic CachedObject returned by the object Storage that overrides the accessor
// methods with a type-casted one.
type CachedTransactionOpinion struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedTransactionOpinion) Retain() *CachedTransactionOpinion {
	return &CachedTransactionOpinion{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedTransactionOpinion) Unwrap() *TransactionOpinion {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*TransactionOpinion)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedTransactionOpinion) Consume(consumer func(opinion *TransactionOpinion), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TransactionOpinion))
	}, forceRelease...)
}

// String returns a human readable version of the CachedTransactionOpinion.
func (c *CachedTransactionOpinion) String() string {
	return stringify.Struct("CachedTransactionOpinion",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictSet //////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictSet is a list of OpinionEssence.
type ConflictSet []OpinionEssence

// hasDecidedLike returns true if the conflict set contains at least one LIKE with LoK >= 2.
func (c ConflictSet) hasDecidedLike() bool {
	for _, opinion := range c {
		if opinion.liked && opinion.levelOfKnowledge > One {
			return true
		}
	}
	return false
}

// anchor returns the oldest opinion with LoK <= 1.
func (c ConflictSet) anchor() (opinion OpinionEssence) {
	oldestTimestamp := time.Unix(1<<63-62135596801, 999999999)
	for _, o := range c {
		if o.levelOfKnowledge <= One && o.timestamp.Before(oldestTimestamp) && (o.timestamp != time.Time{}) {
			oldestTimestamp = o.timestamp
			opinion = OpinionEssence{
				timestamp:        o.timestamp,
				liked:            o.liked,
				levelOfKnowledge: o.levelOfKnowledge,
			}
		}
	}
	return opinion
}

// finalizedAsDisliked returns true if all of the elements of the conflict set have been disliked
// (with a LoK greater than 1).
func (c ConflictSet) finalizedAsDisliked(_ OpinionEssence) bool {
	return !c.hasDecidedLike() && c.anchor() == OpinionEssence{}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TimestampOpinion /////////////////////////////////////////////////////////////////////////////////////////////

const (
	// TimestampOpinionLength defines the length of a TimestampOpinion object (1 byte of opinion, 1 byte of LoK).
	TimestampOpinionLength = tangle.MessageIDLength + 2
)

// TimestampOpinion contains the value of a timestamp opinion as well as its level of knowledge.
type TimestampOpinion struct {
	MessageID tangle.MessageID

	// the essence associated to this opinion.
	OpinionEssence

	likedMutex            sync.RWMutex
	levelOfKnowledgeMutex sync.RWMutex

	objectstorage.StorableObjectFlags
}

// TimestampOpinionFromBytes parses a TimestampOpinion from a byte slice.
func TimestampOpinionFromBytes(bytes []byte) (timestampOpinion *TimestampOpinion, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if timestampOpinion, err = TimestampOpinionFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TimestampOpinion from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TimestampOpinionFromMarshalUtil is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func TimestampOpinionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *TimestampOpinion, err error) {
	readStartOffset := marshalUtil.ReadOffset()

	// read information that are required to identify the TimestampLiked
	result = &TimestampOpinion{}
	if result.MessageID, err = tangle.MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}
	if result.liked, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse liked flag of the opinion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	levelOfKnowledgeUint8, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse level of knowledge of the opinion (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	result.levelOfKnowledge = LevelOfKnowledge(levelOfKnowledgeUint8)

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != TimestampOpinionLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, TimestampOpinionLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TimestampOpinionFromObjectStorage restores a TimestampOpinion from the object Storage.
func TimestampOpinionFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = TimestampOpinionFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse TimestampOpinion from bytes: %w", err)
		return
	}

	return
}

// Equals returns true if the given timestampOpinion is equal to the given x.
func (t *TimestampOpinion) Equals(x *TimestampOpinion) bool {
	return t.MessageID == x.MessageID && t.Liked() == x.Liked() && t.LevelOfKnowledge() == x.LevelOfKnowledge()
}

// Bytes returns the timestamp statement encoded as bytes.
func (t *TimestampOpinion) Bytes() (bytes []byte) {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human readable version of the TimestampOpinion.
func (t *TimestampOpinion) String() string {
	return stringify.Struct("TimestampOpinion",
		stringify.StructField("MessageID", t.MessageID),
		stringify.StructField("Liked", t.Liked()),
		stringify.StructField("LevelOfKnowledge", t.LevelOfKnowledge()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (t *TimestampOpinion) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *TimestampOpinion) ObjectStorageKey() []byte {
	return t.MessageID.Bytes()
}

// ObjectStorageValue marshals the TimestampOpinion into a sequence of bytes that are used as the value part in the
// object Storage.
func (t *TimestampOpinion) ObjectStorageValue() []byte {
	return marshalutil.New(2).
		WriteBool(t.Liked()).
		WriteUint8(uint8(t.LevelOfKnowledge())).
		Bytes()
}

// Liked returns the opinion's liked.
func (o *TimestampOpinion) Liked() bool {
	o.likedMutex.RLock()
	defer o.likedMutex.RUnlock()
	return o.liked
}

// SetLiked sets the opinion's liked.
func (o *TimestampOpinion) SetLiked(l bool) {
	o.likedMutex.Lock()
	defer o.likedMutex.Unlock()
	o.liked = l
	o.SetModified(true)
}

// LevelOfKnowledge returns the opinion's LevelOfKnowledge.
func (o *TimestampOpinion) LevelOfKnowledge() LevelOfKnowledge {
	o.levelOfKnowledgeMutex.RLock()
	defer o.levelOfKnowledgeMutex.RUnlock()
	return o.levelOfKnowledge
}

// SetLevelOfKnowledge sets the opinion's LevelOfKnowledge.
func (o *TimestampOpinion) SetLevelOfKnowledge(lok LevelOfKnowledge) {
	o.levelOfKnowledgeMutex.Lock()
	defer o.levelOfKnowledgeMutex.Unlock()
	o.levelOfKnowledge = lok
	o.SetModified(true)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTimestampOpinion ///////////////////////////////////////////////////////////////////////////////////////

// CachedTimestampOpinion is a wrapper for the generic CachedObject returned by the object Storage that overrides the accessor
// methods with a type-casted one.
type CachedTimestampOpinion struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedTimestampOpinion) Retain() *CachedTimestampOpinion {
	return &CachedTimestampOpinion{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedTimestampOpinion) Unwrap() *TimestampOpinion {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*TimestampOpinion)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedTimestampOpinion) Consume(consumer func(timestampOpinion *TimestampOpinion), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TimestampOpinion))
	}, forceRelease...)
}

// String returns a human readable version of the CachedTimestampOpinion.
func (c *CachedTimestampOpinion) String() string {
	return stringify.Struct("CachedTimestampOpinion",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
