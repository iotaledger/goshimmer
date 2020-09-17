package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
)

// MissingMessage represents a missing message.
type MissingMessage struct {
	objectstorage.StorableObjectFlags

	messageID    message.ID
	missingSince time.Time
}

// NewMissingMessage creates new missing message with the specified messageID.
func NewMissingMessage(messageID message.ID) *MissingMessage {
	return &MissingMessage{
		messageID:    messageID,
		missingSince: time.Now(),
	}
}

// MissingMessageFromBytes parses the given bytes into a MissingMessage.
func MissingMessageFromBytes(bytes []byte) (result *MissingMessage, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMissingMessage(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()
	return
}

// MissingMessageFromObjectStorage creates a MissingMessage from the ObjectStorage.
func MissingMessageFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	result, _, err = MissingMessageFromBytes(byteutils.ConcatBytes(key, data))

	return
}

// ParseMissingMessage parses a MissingMessage from the given marshal util.
func ParseMissingMessage(marshalUtil *marshalutil.MarshalUtil) (result *MissingMessage, err error) {
	result = &MissingMessage{}

	if result.messageID, err = message.ParseID(marshalUtil); err != nil {
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}

	return
}

// MessageID returns the id of the message.
func (missingMessage *MissingMessage) MessageID() message.ID {
	return missingMessage.messageID
}

// MissingSince returns the time since when this message is missing.
func (missingMessage *MissingMessage) MissingSince() time.Time {
	return missingMessage.missingSince
}

// Bytes returns a marshaled version of this MissingMessage.
func (missingMessage *MissingMessage) Bytes() []byte {
	return byteutils.ConcatBytes(missingMessage.ObjectStorageKey(), missingMessage.ObjectStorageValue())
}

// Update update the missing message.
// It should never happen and will panic if called.
func (missingMessage *MissingMessage) Update(other objectstorage.StorableObject) {
	panic("missing messages should never be overwritten and only stored once to optimize IO")
}

// ObjectStorageKey returns the key of the stored missing message.
// This returns the bytes of the messageID of the missing message.
func (missingMessage *MissingMessage) ObjectStorageKey() []byte {
	return missingMessage.messageID[:]
}

// ObjectStorageValue returns the value of the stored missing message.
func (missingMessage *MissingMessage) ObjectStorageValue() (result []byte) {
	result, err := missingMessage.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}
