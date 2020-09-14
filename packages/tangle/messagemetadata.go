package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageID          MessageID
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageID:    messageID,
		receivedTime: time.Now(),
	}
}

// MessageMetadataFromBytes unmarshals the given bytes into a MessageMetadata.
func MessageMetadataFromBytes(bytes []byte) (result *MessageMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MessageMetadataParse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MessageMetadataParse parses the marshalUtil into a MessageMetadata.
// If it successfully parses the marshalUtil, it delegates to MessageMetadataFromStorageKey.
// Else, delegates to UnmarshalObjectStorageValue.
func MessageMetadataParse(marshalUtil *marshalutil.MarshalUtil) (result *MessageMetadata, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return MessageMetadataFromStorageKey(data, nil)
	})
	if parseErr != nil {
		err = parseErr
		return
	}
	result = parsedObject.(*MessageMetadata)
	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// MessageMetadataFromStorageKey unmarshals the stored bytes into a MessageMetadata.
func MessageMetadataFromStorageKey(key []byte, _ []byte) (result objectstorage.StorableObject, consumedBytes int, err error) {
	result = &MessageMetadata{}

	marshalUtil := marshalutil.New(key)
	result.(*MessageMetadata).messageID, err = MessageIDParse(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ReceivedTime returns the time when the message was received.
func (m *MessageMetadata) ReceivedTime() time.Time {
	return m.receivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (m *MessageMetadata) IsSolid() (result bool) {
	m.solidMutex.RLock()
	result = m.solid
	m.solidMutex.RUnlock()

	return
}

// SetSolid sets the message associated with this metadata as solid.
// It returns true if the solid status is modified. False otherwise.
func (m *MessageMetadata) SetSolid(solid bool) (modified bool) {
	m.solidMutex.RLock()
	if m.solid != solid {
		m.solidMutex.RUnlock()

		m.solidMutex.Lock()
		if m.solid != solid {
			m.solid = solid
			if solid {
				m.solidificationTimeMutex.Lock()
				m.solidificationTime = time.Now()
				m.solidificationTimeMutex.Unlock()
			}

			m.SetModified()

			modified = true
		}
		m.solidMutex.Unlock()

	} else {
		m.solidMutex.RUnlock()
	}

	return
}

// SolidificationTime returns the time when the message was marked to be solid.
func (m *MessageMetadata) SolidificationTime() time.Time {
	m.solidificationTimeMutex.RLock()
	defer m.solidificationTimeMutex.RUnlock()

	return m.solidificationTime
}

// ObjectStorageKey returns the key of the stored message metadata object.
// This returns the bytes of the messageID.
func (m *MessageMetadata) ObjectStorageKey() []byte {
	return m.messageID.Bytes()
}

// ObjectStorageValue returns the value of the stored message metadata object.
// This includes the receivedTime, solidificationTime and solid status.
func (m *MessageMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(m.ReceivedTime()).
		WriteTime(m.SolidificationTime()).
		WriteBool(m.IsSolid()).
		Bytes()
}

// UnmarshalObjectStorageValue unmarshals the stored bytes into a messageMetadata.
func (m *MessageMetadata) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)

	if m.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if m.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if m.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Update updates the message metadata.
// This should never happen and will panic if attempted.
func (m *MessageMetadata) Update(other objectstorage.StorableObject) {
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
