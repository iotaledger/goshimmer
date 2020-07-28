package tangle

import (
	"time"

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

// MissingMessageFromStorageKey unmarshals the stored key into a desirable target specified by 0 or 1 optional argument.
// The default target is MissingMessage.
// It unmarshals into the target specified or panics if more than 1 target is specified.
func MissingMessageFromStorageKey(key []byte, optionalTargetObject ...*MissingMessage) (result objectstorage.StorableObject, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &MissingMessage{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingMessageFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	result.(*MissingMessage).messageID, err = message.ParseID(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

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

// UnmarshalObjectStorageValue unmarshals the stored bytes into a missing message.
func (missingMessage *MissingMessage) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	missingMessage.missingSince, err = marshalUtil.ReadTime()
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}
