package fcob

import (
	"fmt"
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

// region Opinion //////////////////////////////////////////////////////////////////////////////////////////////////////

// OpinionEssence contains the essence of an opinion (timestamp, liked, level of knowledge).
type OpinionEssence struct {
	// timestamp is the arrrival/solidification time.
	timestamp time.Time

	// liked is the opinion value.
	liked bool

	// levelOfKnowledge is the degree of certainty of the associated opinion.
	levelOfKnowledge LevelOfKnowledge
}

// Opinion defines the FCoB opinion about a transaction.
type Opinion struct {
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
func (o *Opinion) Timestamp() time.Time {
	o.timestampMutex.RLock()
	defer o.timestampMutex.RUnlock()
	return o.timestamp
}

// SetTimestamp sets the opinion's timestamp.
func (o *Opinion) SetTimestamp(t time.Time) {
	o.timestampMutex.Lock()
	defer o.timestampMutex.Unlock()
	o.timestamp = t
	o.SetModified(true)
}

// Liked returns the opinion's liked.
func (o *Opinion) Liked() bool {
	o.likedMutex.RLock()
	defer o.likedMutex.RUnlock()
	return o.liked
}

// SetLiked sets the opinion's liked.
func (o *Opinion) SetLiked(l bool) {
	o.likedMutex.Lock()
	defer o.likedMutex.Unlock()
	o.liked = l
	o.SetModified(true)
}

// LevelOfKnowledge returns the opinion's LevelOfKnowledge.
func (o *Opinion) LevelOfKnowledge() LevelOfKnowledge {
	o.levelOfKnowledgeMutex.RLock()
	defer o.levelOfKnowledgeMutex.RUnlock()
	return o.levelOfKnowledge
}

// SetLevelOfKnowledge sets the opinion's LevelOfKnowledge.
func (o *Opinion) SetLevelOfKnowledge(lok LevelOfKnowledge) {
	o.levelOfKnowledgeMutex.Lock()
	defer o.levelOfKnowledgeMutex.Unlock()
	o.levelOfKnowledge = lok
	o.SetModified(true)
}

// SetFCOBTime1 sets the opinion's LikedThreshold execution time.
func (o *Opinion) SetFCOBTime1(t time.Time) {
	o.fcobTimeMutex.Lock()
	defer o.fcobTimeMutex.Unlock()
	o.fcobTime1 = t
	o.SetModified()
}

// FCOBTime1 returns the opinion's LikedThreshold execution time.
func (o *Opinion) FCOBTime1() time.Time {
	o.fcobTimeMutex.RLock()
	defer o.fcobTimeMutex.RUnlock()
	return o.fcobTime1
}

// SetFCOBTime2 sets the opinion's LocallyFinalizedThreshold execution time.
func (o *Opinion) SetFCOBTime2(t time.Time) {
	o.fcobTimeMutex.Lock()
	defer o.fcobTimeMutex.Unlock()
	o.fcobTime2 = t
	o.SetModified()
}

// FCOBTime2 returns the opinion's LocallyFinalizedThreshold execution time.
func (o *Opinion) FCOBTime2() time.Time {
	o.fcobTimeMutex.RLock()
	defer o.fcobTimeMutex.RUnlock()
	return o.fcobTime2
}

// Bytes marshals the Opinion into a sequence of bytes.
func (o *Opinion) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human readable version of the Opinion.
func (o *Opinion) String() string {
	return stringify.Struct("Opinion",
		stringify.StructField("transactionID", o.transactionID),
		stringify.StructField("timestamp", o.Timestamp),
		stringify.StructField("liked", o.Liked),
		stringify.StructField("LevelOfKnowledge", o.LevelOfKnowledge),
		stringify.StructField("fcobTime1", o.FCOBTime1()),
		stringify.StructField("fcobTime2", o.FCOBTime2()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (o *Opinion) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *Opinion) ObjectStorageKey() []byte {
	return o.transactionID.Bytes()
}

// ObjectStorageValue marshals the Opinion into a sequence of bytes. The ID is not serialized here as it is
// only used as a key in the ObjectStorage.
func (o *Opinion) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(o.Timestamp()).
		WriteBool(o.Liked()).
		WriteUint8(uint8(o.LevelOfKnowledge())).
		WriteTime(o.FCOBTime1()).
		WriteTime(o.FCOBTime2()).
		Bytes()
}

// OpinionFromMarshalUtil parses an opinion from the given marshal util.
func OpinionFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *Opinion, err error) {
	// parse information
	result = &Opinion{}
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

// OpinionFromObjectStorage restores an Opinion from the ObjectStorage.
func OpinionFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
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
var _ objectstorage.StorableObject = &Opinion{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedOpinion ////////////////////////////////////////////////////////////////////////////////////////////////

// CachedOpinion is a wrapper for the generic CachedObject returned by the object Storage that overrides the accessor
// methods with a type-casted one.
type CachedOpinion struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedOpinion) Retain() *CachedOpinion {
	return &CachedOpinion{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedOpinion) Unwrap() *Opinion {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Opinion)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedOpinion) Consume(consumer func(opinion *Opinion), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Opinion))
	}, forceRelease...)
}

// String returns a human readable version of the CachedOpinion.
func (c *CachedOpinion) String() string {
	return stringify.Struct("CachedOpinion",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region ConflictSet //////////////////////////////////////////////////////////////////////////////////////////////////

// ConflictSet is a list of Opinion.
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
