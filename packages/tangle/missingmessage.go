package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

// region MissingMessage ///////////////////////////////////////////////////////////////////////////////////////////////

// MissingMessage represents a missing message.
type MissingMessage struct {
	objectstorage.StorableObjectFlags

	messageID    MessageID
	missingSince time.Time
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID MessageID) *MissingMessage {
	return &MissingMessage{
		messageID:    messageID,
		missingSince: time.Now(),
	}
}

// MissingMessageFromBytes parses the given bytes into a MissingMessage.
func MissingMessageFromBytes(bytes []byte) (result *MissingMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MissingMessageFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MissingMessageFromMarshalUtil parses a MissingMessage from the given MarshalUtil.
func MissingMessageFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *MissingMessage, err error) {
	result = &MissingMessage{}

	if result.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of missing message: %w", err)
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse missingSince of missing message: %w", err)
		return
	}

	return
}

// MissingMessageFromObjectStorage restores a MissingMessage from the ObjectStorage.
func MissingMessageFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	result, _, err = MissingMessageFromBytes(byteutils.ConcatBytes(key, data))
	if err != nil {
		err = fmt.Errorf("failed to parse missing message from object storage: %w", err)
		return
	}

	return
}

// MessageID returns the id of the message.
func (m *MissingMessage) MessageID() MessageID {
	return m.messageID
}

// MissingSince returns the time since when this message is missing.
func (m *MissingMessage) MissingSince() time.Time {
	return m.missingSince
}

// Bytes returns a marshaled version of this MissingMessage.
func (m *MissingMessage) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
}

// Update update the missing message.
// It should never happen and will panic if called.
func (m *MissingMessage) Update(other objectstorage.StorableObject) {
	panic("missing messages should never be overwritten and only stored once to optimize IO")
}

// ObjectStorageKey returns the key of the stored missing message.
// This returns the bytes of the messageID of the missing message.
func (m *MissingMessage) ObjectStorageKey() []byte {
	return m.messageID[:]
}

// ObjectStorageValue returns the value of the stored missing message.
func (m *MissingMessage) ObjectStorageValue() (result []byte) {
	result, err := m.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedMissingMessage /////////////////////////////////////////////////////////////////////////////////////////

// CachedMissingMessage is a wrapper for the generic CachedObject returned by the object storage that overrides the
// accessor methods with a type-casted one.
type CachedMissingMessage struct {
	objectstorage.CachedObject
}

// ID returns the MissingMessageID of the requested MissingMessage.
func (c *CachedMissingMessage) ID() (id MessageID) {
	id, _, err := MessageIDFromBytes(c.Key())
	if err != nil {
		panic(err)
	}

	return
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedMissingMessage) Retain() *CachedMissingMessage {
	return &CachedMissingMessage{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedMissingMessage) Unwrap() *MissingMessage {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*MissingMessage)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMissingMessage) Consume(consumer func(missingMessage *MissingMessage), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MissingMessage))
	}, forceRelease...)
}

// String returns a human readable version of the CachedMissingMessage.
func (c *CachedMissingMessage) String() string {
	return stringify.Struct("CachedMissingMessage",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
