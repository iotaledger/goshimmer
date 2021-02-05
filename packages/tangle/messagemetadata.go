package tangle

import (
	"fmt"
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/xerrors"
)

// MessageMetadata defines the metadata for a message.
type MessageMetadata struct {
	objectstorage.StorableObjectFlags

	messageID          MessageID
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time
	structureDetails   *markers.StructureDetails
	branchID           ledgerstate.BranchID
	timestampOpinion   TimestampOpinion
	booked             bool
	eligible           bool
	invalid            bool

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	structureDetailsMutex   sync.RWMutex
	branchIDMutex           sync.RWMutex
	timestampOpinionMutex   sync.RWMutex
	bookedMutex             sync.RWMutex
	eligibleMutex           sync.RWMutex
	invalidMutex            sync.RWMutex
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
		err = fmt.Errorf("failed to parse solid flag of message metadata: %w", err)
		return
	}
	if result.structureDetails, err = markers.StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse StructureDetails from MarshalUtil: %w", err)
		return
	}
	if result.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse BranchID from MarshalUtil: %w", err)
		return
	}
	if result.timestampOpinion, err = TimestampOpinionFromMarshalUtil(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse timestampOpinion of message metadata: %w", err)
		return
	}
	if result.eligible, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse eligble flag of message metadata: %w", err)
		return
	}
	if result.booked, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse booked flag of message metadata: %w", err)
		return
	}
	if result.invalid, err = marshalUtil.ReadBool(); err != nil {
		err = fmt.Errorf("failed to parse invalid flag of message metadata: %w", err)
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

// ID returns the MessageID of the Message that this MessageMetadata object belongs to.
func (m *MessageMetadata) ID() MessageID {
	return m.messageID
}

// ReceivedTime returns the time when the message was received.
func (m *MessageMetadata) ReceivedTime() time.Time {
	return m.receivedTime
}

// IsSolid returns true if the message represented by this metadata is solid. False otherwise.
func (m *MessageMetadata) IsSolid() (result bool) {
	m.solidMutex.RLock()
	defer m.solidMutex.RUnlock()
	result = m.solid

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

// SetStructureDetails sets the structureDetails of the message.
func (m *MessageMetadata) SetStructureDetails(structureDetails *markers.StructureDetails) (modified bool) {
	m.structureDetailsMutex.Lock()
	defer m.structureDetailsMutex.Unlock()

	if m.structureDetails != nil {
		return false
	}

	m.structureDetails = structureDetails

	m.SetModified()
	return true
}

// StructureDetails returns the structureDetails of the message.
func (m *MessageMetadata) StructureDetails() *markers.StructureDetails {
	m.structureDetailsMutex.RLock()
	defer m.structureDetailsMutex.RUnlock()

	return m.structureDetails
}

// SetBranchID sets the branch ID of the message.
func (m *MessageMetadata) SetBranchID(ID ledgerstate.BranchID) (modified bool) {
	m.branchIDMutex.Lock()
	defer m.branchIDMutex.Unlock()
	if m.branchID == ID {
		return
	}
	m.branchID = ID
	m.SetModified(true)
	modified = true
	return
}

// BranchID returns the branch ID of the message.
func (m *MessageMetadata) BranchID() ledgerstate.BranchID {
	m.branchIDMutex.RLock()
	defer m.branchIDMutex.RUnlock()
	return m.branchID
}

// IsBooked returns true if the message represented by this metadata is booked. False otherwise.
func (m *MessageMetadata) IsBooked() (result bool) {
	m.bookedMutex.RLock()
	defer m.bookedMutex.RUnlock()
	result = m.booked

	return
}

// IsEligible returns true if the message represented by this metadata is eligible. False otherwise.
func (m *MessageMetadata) IsEligible() (result bool) {
	m.eligibleMutex.RLock()
	defer m.eligibleMutex.RUnlock()
	result = m.eligible

	return
}

// SetBooked sets the message associated with this metadata as booked.
// It returns true if the booked status is modified. False otherwise.
func (m *MessageMetadata) SetBooked(booked bool) (modified bool) {
	m.bookedMutex.Lock()
	defer m.bookedMutex.Unlock()

	if m.booked == booked {
		return false
	}

	m.booked = booked
	m.SetModified()
	modified = true

	return
}

// TimestampOpinion returns the timestampOpinion of the given message metadata.
func (m *MessageMetadata) TimestampOpinion() (timestampOpinion TimestampOpinion) {
	m.timestampOpinionMutex.RLock()
	defer m.timestampOpinionMutex.RUnlock()
	return m.timestampOpinion
}

// SetTimestampOpinion sets the timestampOpinion flag.
// It returns true if the timestampOpinion flag is modified. False otherwise.
func (m *MessageMetadata) SetTimestampOpinion(timestampOpinion TimestampOpinion) (modified bool) {
	m.timestampOpinionMutex.Lock()
	defer m.timestampOpinionMutex.Unlock()

	if m.timestampOpinion.Equal(timestampOpinion) {
		return false
	}

	m.timestampOpinion = timestampOpinion
	m.SetModified()
	return true
}

// SetEligible sets the message associated with this metadata as eligible.
// It returns true if the eligible status is modified. False otherwise.
func (m *MessageMetadata) SetEligible(eligible bool) (modified bool) {
	m.eligibleMutex.Lock()
	defer m.eligibleMutex.Unlock()

	if m.eligible == eligible {
		return false
	}

	m.eligible = eligible
	m.SetModified()
	modified = true

	return
}

// IsInvalid returns true if the message represented by this metadata is invalid. False otherwise.
func (m *MessageMetadata) IsInvalid() (result bool) {
	m.invalidMutex.RLock()
	defer m.invalidMutex.RUnlock()
	result = m.invalid

	return
}

// SetInvalid sets the message associated with this metadata as invalid.
// It returns true if the invalid status is modified. False otherwise.
func (m *MessageMetadata) SetInvalid(invalid bool) (modified bool) {
	m.invalidMutex.Lock()
	defer m.invalidMutex.Unlock()

	if m.invalid == invalid {
		return false
	}

	m.invalid = invalid
	m.SetModified()
	modified = true

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
		Write(m.StructureDetails()).
		Write(m.BranchID()).
		WriteBytes(m.TimestampOpinion().Bytes()).
		WriteBool(m.IsEligible()).
		WriteBool(m.IsBooked()).
		WriteBool(m.IsInvalid()).
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

// ID returns the MessageID of the CachedMessageMetadata.
func (c *CachedMessageMetadata) ID() (messageID MessageID) {
	messageID, _, err := MessageIDFromBytes(c.Key())
	if err != nil {
		panic(err)
	}

	return
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

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedMessageMetadata) Consume(consumer func(messageMetadata *MessageMetadata), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*MessageMetadata))
	}, forceRelease...)
}
