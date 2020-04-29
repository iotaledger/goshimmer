package tangle

import (
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
)

// MissingPayload represents a payload that was referenced through branch or trunk but that is missing in our object
// storage.
type MissingPayload struct {
	objectstorage.StorableObjectFlags

	payloadID    payload.ID
	missingSince time.Time
}

// NewMissingPayload creates an entry for a missing value transfer payload.
func NewMissingPayload(payloadID payload.ID) *MissingPayload {
	return &MissingPayload{
		payloadID:    payloadID,
		missingSince: time.Now(),
	}
}

// MissingPayloadFromBytes unmarshals an entry for a missing value transfer payload from a sequence of bytes.
// It either creates a new entry or fills the optionally provided one with the parsed information.
func MissingPayloadFromBytes(bytes []byte, optionalTargetObject ...*MissingPayload) (result *MissingPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMissingPayload(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseMissingPayload unmarshals a MissingPayload using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseMissingPayload(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*MissingPayload) (result *MissingPayload, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return MissingPayloadFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*MissingPayload)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// MissingPayloadFromStorageKey gets called when we restore an entry for a missing value transfer payload from the storage. The bytes and
// the content will be unmarshaled by an external caller using the binary.ObjectStorageValue interface.
func MissingPayloadFromStorageKey(key []byte, optionalTargetObject ...*MissingPayload) (result *MissingPayload, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &MissingPayload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MissingPayloadFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.payloadID, err = payload.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ID returns the payload id, that is missing.
func (missingPayload *MissingPayload) ID() payload.ID {
	return missingPayload.payloadID
}

// MissingSince returns the time.Time since the transaction was first reported as being missing.
func (missingPayload *MissingPayload) MissingSince() time.Time {
	return missingPayload.missingSince
}

// Bytes marshals the missing payload into a sequence of bytes.
func (missingPayload *MissingPayload) Bytes() []byte {
	return marshalutil.New(payload.IDLength + marshalutil.TIME_SIZE).
		WriteBytes(missingPayload.ObjectStorageKey()).
		WriteBytes(missingPayload.ObjectStorageValue()).
		Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (missingPayload *MissingPayload) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// ObjectStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (missingPayload *MissingPayload) ObjectStorageKey() []byte {
	return missingPayload.payloadID.Bytes()
}

// ObjectStorageValue is required to match the encoding.BinaryMarshaler interface.
func (missingPayload *MissingPayload) ObjectStorageValue() (data []byte) {
	return marshalutil.New(marshalutil.TIME_SIZE).
		WriteTime(missingPayload.missingSince).
		Bytes()
}

// UnmarshalObjectStorageValue is required to match the encoding.BinaryUnmarshaler interface.
func (missingPayload *MissingPayload) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if missingPayload.missingSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &MissingPayload{}
