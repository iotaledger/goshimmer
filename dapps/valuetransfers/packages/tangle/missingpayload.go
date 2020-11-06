package tangle

import (
	"fmt"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// MissingPayload represents a payload that was referenced through parent2 or parent1 but that is missing in our object
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
func MissingPayloadFromBytes(bytes []byte) (result *MissingPayload, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseMissingPayload(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseMissingPayload unmarshals a MissingPayload using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseMissingPayload(marshalUtil *marshalutil.MarshalUtil) (result *MissingPayload, err error) {
	result = &MissingPayload{}

	if result.payloadID, err = payload.ParseID(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse payload id of missing payload: %w", err)
		return
	}
	if result.missingSince, err = marshalUtil.ReadTime(); err != nil {
		err = fmt.Errorf("failed to parse missing time of missing payload: %w", err)
		return
	}

	return
}

// MissingPayloadFromObjectStorage gets called when we restore an entry for a missing value transfer payload from the storage. The bytes and
// the content will be unmarshaled by an external caller using the binary.ObjectStorageValue interface.
func MissingPayloadFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	result, _, err = MissingPayloadFromBytes(byteutils.ConcatBytes(key, data))
	if err != nil {
		err = fmt.Errorf("failed to parse missing payload from object storage: %w", err)
	}

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
	return byteutils.ConcatBytes(missingPayload.ObjectStorageKey(), missingPayload.ObjectStorageValue())
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
	return marshalutil.New(marshalutil.TimeSize).
		WriteTime(missingPayload.MissingSince()).
		Bytes()
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &MissingPayload{}
