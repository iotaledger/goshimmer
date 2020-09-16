package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageID          message.ID
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID message.ID) *MessageMetadata {
	return &MessageMetadata{
		messageID:    messageID,
		receivedTime: time.Now(),
	}
}

// MessageMetadataFromBytes unmarshals the given bytes into a MessageMetadata.
func MessageMetadataFromBytes(bytes []byte) (result *MessageMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMessageMetadata(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseMessageMetadata parses the marshalUtil into a MessageMetadata.
// If it successfully parses the marshalUtil, it delegates to MessageMetadataFromObjectStorage.
// Else, delegates to UnmarshalObjectStorageValue.
func ParseMessageMetadata(marshalUtil *marshalutil.MarshalUtil) (result *MessageMetadata, err error) {
	result = &MessageMetadata{}

	if result.messageID, err = message.ParseID(marshalUtil); err != nil {
		return
	}
	if result.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if result.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}

	return
}

// MessageMetadataFromObjectStorage unmarshals the stored bytes into a MessageMetadata.
func MessageMetadataFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MessageMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		return
	}

	return
}

// ReceivedTime returns the time when the message was received.
func (messageMetadata *MessageMetadata) ReceivedTime() time.Time {
	return messageMetadata.receivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (messageMetadata *MessageMetadata) IsSolid() (result bool) {
	messageMetadata.solidMutex.RLock()
	result = messageMetadata.solid
	messageMetadata.solidMutex.RUnlock()

	return
}

// SetSolid sets the message associated with this metadata as solid.
// It returns true if the solid status is modified. False otherwise.
func (messageMetadata *MessageMetadata) SetSolid(solid bool) (modified bool) {
	messageMetadata.solidMutex.RLock()
	if messageMetadata.solid != solid {
		messageMetadata.solidMutex.RUnlock()

		messageMetadata.solidMutex.Lock()
		if messageMetadata.solid != solid {
			messageMetadata.solid = solid
			if solid {
				messageMetadata.solidificationTimeMutex.Lock()
				messageMetadata.solidificationTime = time.Now()
				messageMetadata.solidificationTimeMutex.Unlock()
			}

			messageMetadata.SetModified()

			modified = true
		}
		messageMetadata.solidMutex.Unlock()

	} else {
		messageMetadata.solidMutex.RUnlock()
	}

	return
}

// SolidificationTime returns the time when the message was marked to be solid.
func (messageMetadata *MessageMetadata) SolidificationTime() time.Time {
	messageMetadata.solidificationTimeMutex.RLock()
	defer messageMetadata.solidificationTimeMutex.RUnlock()

	return messageMetadata.solidificationTime
}

// Bytes returns a marshaled version of the whole MessageMetadata object.
func (messageMetadata *MessageMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(messageMetadata.ObjectStorageKey(), messageMetadata.ObjectStorageValue())
}

// ObjectStorageKey returns the key of the stored message metadata object.
// This returns the bytes of the messageID.
func (messageMetadata *MessageMetadata) ObjectStorageKey() []byte {
	return messageMetadata.messageID.Bytes()
}

// ObjectStorageValue returns the value of the stored message metadata object.
// This includes the receivedTime, solidificationTime and solid status.
func (messageMetadata *MessageMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(messageMetadata.ReceivedTime()).
		WriteTime(messageMetadata.SolidificationTime()).
		WriteBool(messageMetadata.IsSolid()).
		Bytes()
}

// Update updates the message metadata.
// This should never happen and will panic if attempted.
func (messageMetadata *MessageMetadata) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

var _ objectstorage.StorableObject = &MessageMetadata{}

// CachedMessageMetadata is a wrapper for stored cached object that represents a message metadata.
type CachedMessageMetadata struct {
	objectstorage.CachedObject
}

// Retain registers a new consumer for the cached message metadata.
func (cachedMessageMetadata *CachedMessageMetadata) Retain() *CachedMessageMetadata {
	return &CachedMessageMetadata{cachedMessageMetadata.CachedObject.Retain()}
}

// Unwrap returns the underlying stored message metadata wrapped by the CachedMessageMetadata.
// If the stored object cannot be cast to MessageMetadata or is deleted, it returns nil.
func (cachedMessageMetadata *CachedMessageMetadata) Unwrap() *MessageMetadata {
	untypedObject := cachedMessageMetadata.Get()
	if untypedObject == nil {
		return nil
	}
	typedObject := untypedObject.(*MessageMetadata)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}
	return typedObject
}
