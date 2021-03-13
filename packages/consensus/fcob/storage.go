package fcob

import (
	"sync"
	"time"

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

// Storage is a component of the ConsensusMechanism that encapsulates the storage related methods.
type Storage struct {
	store                   kvstore.KVStore
	opinionStorage          *objectstorage.ObjectStorage
	timestampOpinionStorage *objectstorage.ObjectStorage
}

// NewStorage is the constructor for a Storage.
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

// Opinion returns the Opinion associated with given TransactionID.
func (s *Storage) Opinion(transactionID ledgerstate.TransactionID) (cachedOpinion *CachedOpinion) {
	return &CachedOpinion{CachedObject: s.opinionStorage.Load(transactionID.Bytes())}
}

// TimestampOpinion returns the TimestampOpinion associated with given MessageID.
func (s *Storage) TimestampOpinion(messageID tangle.MessageID) (cachedTimestampOpinion *CachedTimestampOpinion) {
	return &CachedTimestampOpinion{CachedObject: s.timestampOpinionStorage.Load(messageID.Bytes())}
}

// StoreTimestampOpinion stores the TimestampOpinion in the object storage. It returns true if it was stored or updated.
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

// Shutdown shuts down the Storage and causes its content to be persisted to the disk.
func (s *Storage) Shutdown() {
	s.opinionStorage.Shutdown()
	s.timestampOpinionStorage.Shutdown()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Object Storage Parameters ////////////////////////////////////////////////////////////////////////////////////

const (
	// PrefixOpinion defines the storage prefix for the opinion storage.
	PrefixOpinion byte = iota

	// PrefixTimestampOpinion defines the storage prefix for the timestamp opinion storage.
	PrefixTimestampOpinion

	// cacheTime defines the duration that the object storage caches objects.
	cacheTime = 2 * time.Second
)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region MessageMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	id                          tangle.MessageID
	payloadOpinionFormed        bool
	payloadOpinionFormedMutex   sync.RWMutex
	timestampOpinionFormed      bool
	timestampOpinionFormedMutex sync.RWMutex
	messageOpinionFormed        bool
	messageOpinionFormedMutex   sync.RWMutex

	objectstorage.StorableObjectFlags
}

// MessageMetadataFromBytes unmarshals a MessageMetadata object from a sequence of bytes.
func MessageMetadataFromBytes(bytes []byte) (messageMetadata *MessageMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if messageMetadata, err = MessageMetadataFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MessageMetadataFromMarshalUtil unmarshals a MessageMetadata object using a MarshalUtil (for easier unmarshaling).
func MessageMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (messageMetadata *MessageMetadata, err error) {
	messageMetadata = &MessageMetadata{}
	if messageMetadata.id, err = tangle.MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse MessageID: %w", err)
		return
	}
	if messageMetadata.payloadOpinionFormed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse payloadOpinionFormed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if messageMetadata.timestampOpinionFormed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse timestampOpinionFormed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if messageMetadata.messageOpinionFormed, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse messageOpinionFormed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// MessageMetadataFromObjectStorage restores a MessageMetadata object from the object storage.
func MessageMetadataFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MessageMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse MessageMetadata from bytes: %w", err)
		return
	}

	return
}

// NewMessageMetadata is the constructor of the MessageMetadata object.
func NewMessageMetadata(messageID tangle.MessageID) *MessageMetadata {
	return &MessageMetadata{
		id: messageID,
	}
}

// ID returns the MessageID that this metadata object belongs to.
func (m *MessageMetadata) ID() tangle.MessageID {
	return m.id
}

// PayloadOpinionFormed returns the payloadOpinionFormed flag of the MessageMetadata.
func (m *MessageMetadata) PayloadOpinionFormed() bool {
	m.payloadOpinionFormedMutex.RLock()
	defer m.payloadOpinionFormedMutex.RUnlock()

	return m.payloadOpinionFormed
}

// SetPayloadOpinionFormed sets the payloadOpinionFormed flag to the given value. It returns true if the value has been
// updated.
func (m *MessageMetadata) SetPayloadOpinionFormed(payloadOpinionFormed bool) (modified bool) {
	m.payloadOpinionFormedMutex.Lock()
	defer m.payloadOpinionFormedMutex.Unlock()

	if m.payloadOpinionFormed == payloadOpinionFormed {
		return
	}

	m.payloadOpinionFormed = payloadOpinionFormed
	modified = true

	m.SetModified()
	m.Persist()

	return
}

// TimestampOpinionFormed returns the timestampOpinionFormed flag of the MessageMetadata.
func (m *MessageMetadata) TimestampOpinionFormed() bool {
	m.timestampOpinionFormedMutex.RLock()
	defer m.timestampOpinionFormedMutex.RUnlock()

	return m.timestampOpinionFormed
}

// SetTimestampOpinionFormed sets the timestampOpinionFormed flag to the given value. It returns true if the value has
// been updated.
func (m *MessageMetadata) SetTimestampOpinionFormed(timestampOpinionFormed bool) (modified bool) {
	m.timestampOpinionFormedMutex.Lock()
	defer m.timestampOpinionFormedMutex.Unlock()

	if m.timestampOpinionFormed == timestampOpinionFormed {
		return
	}

	m.timestampOpinionFormed = timestampOpinionFormed
	modified = true

	m.SetModified()
	m.Persist()

	return
}

// MessageOpinionFormed returns the messageOpinionFormed flag of the MessageMetadata.
func (m *MessageMetadata) MessageOpinionFormed() bool {
	m.messageOpinionFormedMutex.RLock()
	defer m.messageOpinionFormedMutex.RUnlock()

	return m.messageOpinionFormed
}

// SetMessageOpinionFormed sets the messageOpinionFormed flag to the given value. It returns true if the value has been
// updated.
func (m *MessageMetadata) SetMessageOpinionFormed(messageOpinionFormed bool) (modified bool) {
	m.messageOpinionFormedMutex.Lock()
	defer m.messageOpinionFormedMutex.Unlock()

	if m.messageOpinionFormed == messageOpinionFormed {
		return
	}

	m.messageOpinionFormed = messageOpinionFormed
	modified = true

	m.SetModified()
	m.Persist()

	return
}

// Bytes returns a marshaled version of the MessageMetadata.
func (m *MessageMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// String returns a human readable version of the MessageMetadata.
func (m *MessageMetadata) String() string {
	return stringify.Struct("MessageMetadata",
		stringify.StructField("payloadOpinionFormed", m.PayloadOpinionFormed()),
		stringify.StructField("timestampOpinionFormed", m.TimestampOpinionFormed()),
		stringify.StructField("messageOpinionFormed", m.MessageOpinionFormed()),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (m *MessageMetadata) Update(objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (m *MessageMetadata) ObjectStorageKey() []byte {
	return m.id.Bytes()
}

// ObjectStorageValue marshals the AggregatedBranch into a sequence of bytes that are used as the value part in the
// object storage.
func (m *MessageMetadata) ObjectStorageValue() []byte {
	return marshalutil.New(3 * marshalutil.BoolSize).
		WriteBool(m.PayloadOpinionFormed()).
		WriteBool(m.TimestampOpinionFormed()).
		WriteBool(m.MessageOpinionFormed()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &MessageMetadata{}

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

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (t *TimestampOpinion) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *TimestampOpinion) ObjectStorageKey() []byte {
	return t.MessageID.Bytes()
}

// ObjectStorageValue marshals the ConflictBranch into a sequence of bytes that are used as the value part in the
// object storage.
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
