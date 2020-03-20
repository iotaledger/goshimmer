package transfermetadata

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/transfer/id"
)

// TransferMetadata is a container for the metadata of a transfer.
// It is used to store the information in the database.
type TransferMetadata struct {
	objectstorage.StorableObjectFlags

	transferId         id.Id
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// New creates an empty container for the metadata of a value transfer.
func New(transferId id.Id) *TransferMetadata {
	return &TransferMetadata{
		transferId: transferId,
	}
}

// FromBytes unmarshals a container with the metadata of a transfer from a sequence of bytes.
// It either creates a new container or fills the optionally provided container with the parsed information.
func FromBytes(bytes []byte, optionalTargetObject ...*TransferMetadata) (result *TransferMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &TransferMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to FromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.transferId, err = id.Parse(marshalUtil); err != nil {
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

// FromStorage gets called when we restore transfer metadata from the storage. The bytes and the content will be
// unmarshaled by an external caller using the binary.MarshalBinary interface.
func FromStorage(idBytes []byte) objectstorage.StorableObject {
	result := &TransferMetadata{}

	var err error
	if result.transferId, err = id.Parse(marshalutil.New(idBytes)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func Parse(marshalUtil *marshalutil.MarshalUtil) (*TransferMetadata, error) {
	if transferMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return FromBytes(data) }); err != nil {
		return nil, err
	} else {
		return transferMetadata.(*TransferMetadata), nil
	}
}

// TransferId return the id of the transfer that this metadata is associated to.
func (transferMetadata *TransferMetadata) TransferId() id.Id {
	return transferMetadata.transferId
}

// IsSolid returns true if the transfer has been marked as solid.
func (transferMetadata *TransferMetadata) IsSolid() (result bool) {
	transferMetadata.solidMutex.RLock()
	result = transferMetadata.solid
	transferMetadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a transfer as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (transferMetadata *TransferMetadata) SetSolid(solid bool) (modified bool) {
	transferMetadata.solidMutex.RLock()
	if transferMetadata.solid != solid {
		transferMetadata.solidMutex.RUnlock()

		transferMetadata.solidMutex.Lock()
		if transferMetadata.solid != solid {
			transferMetadata.solid = solid
			if solid {
				transferMetadata.solidificationTimeMutex.Lock()
				transferMetadata.solidificationTime = time.Now()
				transferMetadata.solidificationTimeMutex.Unlock()
			}

			transferMetadata.SetModified()

			modified = true
		}
		transferMetadata.solidMutex.Unlock()

	} else {
		transferMetadata.solidMutex.RUnlock()
	}

	return
}

// SoldificationTime returns the time when the transfer was marked to be solid.
func (transferMetadata *TransferMetadata) SoldificationTime() time.Time {
	transferMetadata.solidificationTimeMutex.RLock()
	defer transferMetadata.solidificationTimeMutex.RUnlock()

	return transferMetadata.solidificationTime
}

// Bytes marshals the metadata into a sequence of bytes.
func (transferMetadata *TransferMetadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(transferMetadata.transferId.Bytes())
	marshalUtil.WriteTime(transferMetadata.solidificationTime)
	marshalUtil.WriteBool(transferMetadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (transferMetadata *TransferMetadata) String() string {
	return stringify.Struct("TransferMetadata",
		stringify.StructField("payloadId", transferMetadata.TransferId()),
		stringify.StructField("solid", transferMetadata.IsSolid()),
		stringify.StructField("solidificationTime", transferMetadata.SoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (transferMetadata *TransferMetadata) GetStorageKey() []byte {
	return transferMetadata.transferId.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (transferMetadata *TransferMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary is required to match the encoding.BinaryMarshaler interface.
func (transferMetadata *TransferMetadata) MarshalBinary() ([]byte, error) {
	return transferMetadata.Bytes(), nil
}

// UnmarshalBinary is required to match the encoding.BinaryUnmarshaler interface.
func (transferMetadata *TransferMetadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = FromBytes(data, transferMetadata)

	return
}
