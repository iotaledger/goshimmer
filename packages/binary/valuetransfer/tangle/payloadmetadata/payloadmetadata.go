package payloadmetadata

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
)

// PayloadMetadata is a container for the metadata of a value transfer payload.
// It is used to store the information in the database.
type PayloadMetadata struct {
	objectstorage.StorableObjectFlags

	payloadId          payloadid.Id
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// New creates an empty container for the metadata of a value transfer payload.
func New(payloadId payloadid.Id) *PayloadMetadata {
	return &PayloadMetadata{
		payloadId: payloadId,
	}
}

// FromBytes unmarshals a container with the metadata of a value transfer payload from a sequence of bytes.
// It either creates a new container or fills the optionally provided container with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*PayloadMetadata) (result *PayloadMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &PayloadMetadata{}
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
	if result.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if result.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// FromStorage gets called when we restore transaction metadata from the storage. The bytes and the content will be
// unmarshaled by an external caller using the binary.MarshalBinary interface.
func FromStorage(id []byte) objectstorage.StorableObject {
	result := &PayloadMetadata{}

	var err error
	if result.payloadId, err = payloadid.Parse(marshalutil.New(id)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*PayloadMetadata, error) {
	if payloadMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return payloadMetadata.(*PayloadMetadata), nil
	}
}

// GetPayloadId return the id of the payload that this metadata is associated to.
func (payloadMetadata *PayloadMetadata) GetPayloadId() payloadid.Id {
	return payloadMetadata.payloadId
}

// IsSolid returns true of the payload has been marked as solid.
func (payloadMetadata *PayloadMetadata) IsSolid() (result bool) {
	payloadMetadata.solidMutex.RLock()
	result = payloadMetadata.solid
	payloadMetadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a payload as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (payloadMetadata *PayloadMetadata) SetSolid(solid bool) (modified bool) {
	payloadMetadata.solidMutex.RLock()
	if payloadMetadata.solid != solid {
		payloadMetadata.solidMutex.RUnlock()

		payloadMetadata.solidMutex.Lock()
		if payloadMetadata.solid != solid {
			payloadMetadata.solid = solid
			if solid {
				payloadMetadata.solidificationTimeMutex.Lock()
				payloadMetadata.solidificationTime = time.Now()
				payloadMetadata.solidificationTimeMutex.Unlock()
			}

			payloadMetadata.SetModified()

			modified = true
		}
		payloadMetadata.solidMutex.Unlock()

	} else {
		payloadMetadata.solidMutex.RUnlock()
	}

	return
}

// GetSoldificationTime returns the time when the payload was marked to be solid.
func (payloadMetadata *PayloadMetadata) GetSoldificationTime() time.Time {
	payloadMetadata.solidificationTimeMutex.RLock()
	defer payloadMetadata.solidificationTimeMutex.RUnlock()

	return payloadMetadata.solidificationTime
}

// Bytes marshals the metadata into a sequence of bytes.
func (payloadMetadata *PayloadMetadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(payloadMetadata.payloadId.Bytes())
	marshalUtil.WriteTime(payloadMetadata.solidificationTime)
	marshalUtil.WriteBool(payloadMetadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (payloadMetadata *PayloadMetadata) String() string {
	return stringify.Struct("PayloadMetadata",
		stringify.StructField("payloadId", payloadMetadata.GetPayloadId()),
		stringify.StructField("solid", payloadMetadata.IsSolid()),
		stringify.StructField("solidificationTime", payloadMetadata.GetSoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (payloadMetadata *PayloadMetadata) GetStorageKey() []byte {
	return payloadMetadata.payloadId.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (payloadMetadata *PayloadMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary is required to match the encoding.BinaryMarshaler interface.
func (payloadMetadata *PayloadMetadata) MarshalBinary() ([]byte, error) {
	return payloadMetadata.Bytes(), nil
}

// UnmarshalBinary is required to match the encoding.BinaryUnmarshaler interface.
func (payloadMetadata *PayloadMetadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, payloadMetadata)

	return
}
