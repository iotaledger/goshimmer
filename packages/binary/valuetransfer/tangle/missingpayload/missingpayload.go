package missingpayload

import (
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
)

// MissingPayload represents a payload that was referenced through branch or trunk but that is missing in our object
// storage.
type MissingPayload struct {
	objectstorage.StorableObjectFlags

	payloadId    payloadid.Id
	missingSince time.Time
}

// New creates an entry for a missing value transfer payload.
func New(payloadId payloadid.Id) *MissingPayload {
	return &MissingPayload{
		payloadId:    payloadId,
		missingSince: time.Now(),
	}
}

// FromBytes unmarshals an entry for a missing value transfer payload from a sequence of bytes.
// It either creates a new entry or fills the optionally provided one with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*MissingPayload) (result *MissingPayload, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &MissingPayload{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.payloadId, err = payloadid.Parse(marshalUtil); err != nil {
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromStorage gets called when we restore an entry for a missing value transfer payload from the storage. The bytes and
// the content will be unmarshaled by an external caller using the binary.MarshalBinary interface.
func FromStorage([]byte) objectstorage.StorableObject {
	return &MissingPayload{}
}

// GetId returns the payload id, that is missing.
func (missingPayload *MissingPayload) GetId() payloadid.Id {
	return missingPayload.payloadId
}

// GetMissingSince returns the time.Time since the transaction was first reported as being missing.
func (missingPayload *MissingPayload) GetMissingSince() time.Time {
	return missingPayload.missingSince
}

// Bytes marshals the missing payload into a sequence of bytes.
func (missingPayload *MissingPayload) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(missingPayload.payloadId.Bytes())
	marshalUtil.WriteTime(missingPayload.missingSince)

	return marshalUtil.Bytes()
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (missingPayload *MissingPayload) GetStorageKey() []byte {
	return missingPayload.payloadId.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (missingPayload *MissingPayload) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

// MarshalBinary is required to match the encoding.BinaryMarshaler interface.
func (missingPayload *MissingPayload) MarshalBinary() (data []byte, err error) {
	return missingPayload.Bytes(), nil
}

// UnmarshalBinary is required to match the encoding.BinaryUnmarshaler interface.
func (missingPayload *MissingPayload) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, missingPayload)

	return
}
