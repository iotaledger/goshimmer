package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/hive.go/byteutils"
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
	booked             bool

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	bookedMutex             sync.RWMutex
}

// NewMessageMetadata creates a new MessageMetadata from the specified messageID.
func NewMessageMetadata(messageID MessageID) *MessageMetadata {
	return &MessageMetadata{
		messageID:    messageID,
		receivedTime: clock.SyncedTime(),
	}
}

// MessageMetadataFromBytes unmarshals the given bytes into a MessageMetadata.
func MessageMetadataFromBytes(bytes []byte) (result *MessageMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = MessageMetadataFromMarshalUtil(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// MessageMetadataFromMarshalUtil parses a Message from the given MarshalUtil.
func MessageMetadataFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (result *MessageMetadata, err error) {
	result = &MessageMetadata{}

	if result.messageID, err = MessageIDFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse message ID of message metadata: %w", err)
		return
	}
	if result.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse received time of message metadata: %w", err)
		return
	}
	if result.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse solidification time of message metadata: %w", err)
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse 'solid' of message metadata: %w", err)
		return
	}

	return
}

// MessageMetadataFromObjectStorage restores a MessageMetadata object from the ObjectStorage.
func MessageMetadataFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MessageMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = fmt.Errorf("failed to parse message metadata from object storage: %w", err)
		return
	}

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
				m.solidificationTime = clock.SyncedTime()
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

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.bookedMutex.RLock()
	result = m.booked
	m.bookedMutex.RUnlock()

	return
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.bookedMutex.RLock()
	if m.booked != booked {
		m.bookedMutex.RUnlock()

		m.bookedMutex.Lock()
		if m.booked != booked {
			m.booked = booked
			m.SetModified()
			modified = true
		}
		m.bookedMutex.Unlock()

	} else {
		m.bookedMutex.RUnlock()
	}

	return
}

// Bytes returns a marshaled version of the whole MessageMetadata object.
func (m *MessageMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(m.ObjectStorageKey(), m.ObjectStorageValue())
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
func (c *CachedMessageMetadata) Retain() *CachedMessageMetadata {
	return &CachedMessageMetadata{c.CachedObject.Retain()}
}

// Unwrap returns the underlying stored message metadata wrapped by the CachedMessageMetadata.
// If the stored object cannot be cast to MessageMetadata or is deleted, it returns nil.
func (c *CachedMessageMetadata) Unwrap() *MessageMetadata {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}
	typedObject := untypedObject.(*MessageMetadata)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}
	return typedObject
}
