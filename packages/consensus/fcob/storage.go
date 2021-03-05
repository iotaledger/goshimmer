package fcob

import (
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"golang.org/x/xerrors"
)

// region Storage //////////////////////////////////////////////////////////////////////////////////////////////////////

type Storage struct {
	store                   kvstore.KVStore
	opinionStorage          *objectstorage.ObjectStorage
	timestampOpinionStorage *objectstorage.ObjectStorage
}

func NewStorage(store kvstore.KVStore) (storage *Storage) {
	osFactory := objectstorage.NewFactory(store, database.PrefixFCOB)

	return &Storage{
		store:                   store,
		opinionStorage:          osFactory.New(PrefixOpinion, OpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
		timestampOpinionStorage: osFactory.New(PrefixTimestampOpinion, TimestampOpinionFromObjectStorage, objectstorage.CacheTime(cacheTime), objectstorage.LeakDetectionEnabled(false)),
	}
}

// OpinionEssence returns the OpinionEssence (i.e., a copy of the triple{timestamp, liked, levelOfKnowledge})
// of given transactionID.
func (s *Storage) OpinionEssence(transactionID ledgerstate.TransactionID) (opinion OpinionEssence) {
	(&CachedOpinion{CachedObject: s.opinionStorage.Load(transactionID.Bytes())}).Consume(func(storedOpinion *Opinion) {
		opinion = storedOpinion.OpinionEssence
	})

	return
}

// CachedOpinion returns the CachedOpinion of the given TransactionID.
func (s *Storage) Opinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
	return &CachedOpinion{CachedObject: s.opinionStorage.Load(transactionID.Bytes())}
}

// TimestampOpinion returns the timestampOpinion of the given message metadata.
func (s *Storage) TimestampOpinion(messageID tangle.MessageID) (timestampOpinion *TimestampOpinion) {
	(&CachedTimestampOpinion{CachedObject: s.timestampOpinionStorage.Load(messageID.Bytes())}).Consume(func(opinion *TimestampOpinion) {
		timestampOpinion = opinion
	})

	return
}

// SetTimestampOpinion sets the timestampOpinion flag.
// It returns true if the timestampOpinion flag is modified. False otherwise.
func (s *Storage) StoreTimestampOpinion(timestampOpinion *TimestampOpinion) (modified bool) {
	cachedTimestampOpinion := &CachedTimestampOpinion{CachedObject: s.timestampOpinionStorage.ComputeIfAbsent(timestampOpinion.MessageID.Bytes(), func(key []byte) objectstorage.StorableObject {
		timestampOpinion.SetModified()
		timestampOpinion.Persist()
		modified = true

		return timestampOpinion
	})}

	if modified {
		cachedTimestampOpinion.Release()
		return
	}

	cachedTimestampOpinion.Consume(func(loadedTimestampOpinion *TimestampOpinion) {
		if loadedTimestampOpinion.Equals(timestampOpinion) {
			return
		}

		loadedTimestampOpinion.LoK = timestampOpinion.LoK
		loadedTimestampOpinion.Value = timestampOpinion.Value

		timestampOpinion.SetModified()
		timestampOpinion.Persist()
		modified = true
	})

	return
}

func (s *Storage) Shutdown() {
	s.opinionStorage.Shutdown()
	s.timestampOpinionStorage.Shutdown()
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
	Value     opinion.Opinion
	LoK       LevelOfKnowledge

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

	// read information that are required to identify the TimestampOpinion
	result = &TimestampOpinion{}
	if result.MessageID, err = tangle.MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MessageID from MarshalUtil: %w", err)
		return
	}
	opinionByte, e := marshalUtil.ReadByte()
	if e != nil {
		err = xerrors.Errorf("failed to parse opinion from bytes: %w", e)
		return
	}
	result.Value = opinion.Opinion(opinionByte)

	loKUint8, err := marshalUtil.ReadUint8()
	if err != nil {
		err = xerrors.Errorf("failed to parse Level of Knowledge from bytes: %w", err)
		return
	}
	result.LoK = LevelOfKnowledge(loKUint8)

	// return the number of bytes we processed
	parsedBytes := marshalUtil.ReadOffset() - readStartOffset
	if parsedBytes != TimestampOpinionLength {
		err = xerrors.Errorf("parsed bytes (%d) did not match expected size (%d): %w", parsedBytes, TimestampOpinionLength, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// TimestampOpinionFromObjectStorage restores a TimestampOpinion from the object storage.
func TimestampOpinionFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = TimestampOpinionFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse TimestampOpinion from bytes: %w", err)
		return
	}

	return
}

// Equals returns true if the given timestampOpinion is equal to the given x.
func (t *TimestampOpinion) Equals(x *TimestampOpinion) bool {
	return t.MessageID == x.MessageID && t.Value == x.Value && t.LoK == x.LoK
}

// Bytes returns the timestamp statement encoded as bytes.
func (t *TimestampOpinion) Bytes() (bytes []byte) {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human readable version of the TimestampOpinion.
func (t *TimestampOpinion) String() string {
	return stringify.Struct("TimestampOpinion",
		stringify.StructField("MessageID", t.MessageID),
		stringify.StructField("Value", t.Value),
		stringify.StructField("LoK", t.LoK),
	)
}

func (t *TimestampOpinion) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

func (t *TimestampOpinion) ObjectStorageKey() []byte {
	return t.MessageID.Bytes()
}

func (t *TimestampOpinion) ObjectStorageValue() []byte {
	return marshalutil.New(2).
		WriteByte(byte(t.Value)).
		WriteUint8(uint8(t.LoK)).
		Bytes()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedTimestampOpinion ///////////////////////////////////////////////////////////////////////////////////////

// CachedTimestampOpinion is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
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
