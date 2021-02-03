package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
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

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
	structureDetailsMutex   sync.RWMutex
	branchIDMutex           sync.RWMutex
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
		err = xerrors.Errorf("failed to parse message ID of message metadata: %w", err)
		return
	}
	if result.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse received time of message metadata (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if result.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = xerrors.Errorf("failed to parse solidification time of message metadata (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse 'solid' of message metadata (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	// determine if message metadata has structureDetails set and parse
	var hasStructureDetails bool
	if hasStructureDetails, err = marshalUtil.ReadBool(); err != nil {
		err = xerrors.Errorf("failed to parse 'hasStructureDetails' of message metadata (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if hasStructureDetails {
		if result.structureDetails, err = markers.StructureDetailsFromMarshalUtil(marshalUtil); err != nil {
			err = xerrors.Errorf("failed to parse 'structureDetails' from MarshalUtil: %w", err)
			return
		}
	}
	if result.branchID, err = ledgerstate.BranchIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse branch ID of message metadata: %w", err)
		return
	}

	return
}

// MessageMetadataFromObjectStorage restores a MessageMetadata object from the ObjectStorage.
func MessageMetadataFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = MessageMetadataFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse message metadata from object storage: %w", err)
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
	// TODO: we need to verify that it cannot lead to read/write loss if we're still using structureDetails somewhere
	m.structureDetailsMutex.Lock()
	defer m.structureDetailsMutex.Unlock()
	structureDetailsBytes := []byte{0}
	if m.structureDetails != nil {
		structureDetailsBytes = byteutils.ConcatBytes([]byte{1}, m.structureDetails.Bytes())
	}

	return marshalutil.New().
		WriteTime(m.ReceivedTime()).
		WriteTime(m.SolidificationTime()).
		WriteBool(m.IsSolid()).
		WriteBytes(structureDetailsBytes).
		WriteBytes(m.branchID.Bytes()).
		Bytes()
}

// Update updates the message metadata.
// This should never happen and will panic if attempted.
func (m *MessageMetadata) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// String returns a human readable version of the MessageMetadata.
func (m *MessageMetadata) String() string {
	return stringify.Struct("MessageMetadata",
		stringify.StructField("ID", m.messageID),
		stringify.StructField("ReceivedTime", m.ReceivedTime()),
		stringify.StructField("IsSolid", m.IsSolid()),
		stringify.StructField("SolidificationTime", m.SolidificationTime()),
		stringify.StructField("StructureDetails", m.StructureDetails()),
		stringify.StructField("branchID", m.branchID),
	)
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
