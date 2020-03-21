package payload

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

// Metadata is a container for the metadata of a value transfer payload.
// It is used to store the information in the database.
type Metadata struct {
	objectstorage.StorableObjectFlags

	payloadId          Id
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewMetadata creates an empty container for the metadata of a value transfer payload.
func NewMetadata(payloadId Id) *Metadata {
	return &Metadata{
		payloadId: payloadId,
	}
}

// MetadataFromBytes unmarshals a container with the metadata of a value transfer payload from a sequence of bytes.
// It either creates a new container or fills the optionally provided container with the parsed information.
func MetadataFromBytes(bytes []byte, optionalTargetObject ...*Metadata) (result *Metadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Metadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to MetadataFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.payloadId, err = ParseId(marshalUtil); err != nil {
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

// MetadataFromStorage gets called when we restore transaction metadata from the storage. The bytes and the content will be
// unmarshaled by an external caller using the binary.MarshalBinary interface.
func MetadataFromStorage(id []byte) objectstorage.StorableObject {
	result := &Metadata{}

	var err error
	if result.payloadId, err = ParseId(marshalutil.New(id)); err != nil {
		panic(err)
	}

	return result
}

// ParseMetadata is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParseMetadata(marshalUtil *marshalutil.MarshalUtil) (*Metadata, error) {
	if payloadMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return MetadataFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return payloadMetadata.(*Metadata), nil
	}
}

// GetPayloadId return the id of the payload that this metadata is associated to.
func (metadata *Metadata) GetPayloadId() Id {
	return metadata.payloadId
}

// IsSolid returns true if the payload has been marked as solid.
func (metadata *Metadata) IsSolid() (result bool) {
	metadata.solidMutex.RLock()
	result = metadata.solid
	metadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a payload as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (metadata *Metadata) SetSolid(solid bool) (modified bool) {
	metadata.solidMutex.RLock()
	if metadata.solid != solid {
		metadata.solidMutex.RUnlock()

		metadata.solidMutex.Lock()
		if metadata.solid != solid {
			metadata.solid = solid
			if solid {
				metadata.solidificationTimeMutex.Lock()
				metadata.solidificationTime = time.Now()
				metadata.solidificationTimeMutex.Unlock()
			}

			metadata.SetModified()

			modified = true
		}
		metadata.solidMutex.Unlock()

	} else {
		metadata.solidMutex.RUnlock()
	}

	return
}

// GetSoldificationTime returns the time when the payload was marked to be solid.
func (metadata *Metadata) GetSoldificationTime() time.Time {
	metadata.solidificationTimeMutex.RLock()
	defer metadata.solidificationTimeMutex.RUnlock()

	return metadata.solidificationTime
}

// Bytes marshals the metadata into a sequence of bytes.
func (metadata *Metadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(metadata.payloadId.Bytes())
	marshalUtil.WriteTime(metadata.solidificationTime)
	marshalUtil.WriteBool(metadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (metadata *Metadata) String() string {
	return stringify.Struct("Metadata",
		stringify.StructField("payloadId", metadata.GetPayloadId()),
		stringify.StructField("solid", metadata.IsSolid()),
		stringify.StructField("solidificationTime", metadata.GetSoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (metadata *Metadata) GetStorageKey() []byte {
	return metadata.payloadId.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (metadata *Metadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary is required to match the encoding.BinaryMarshaler interface.
func (metadata *Metadata) MarshalBinary() ([]byte, error) {
	return metadata.Bytes(), nil
}

// UnmarshalBinary is required to match the encoding.BinaryUnmarshaler interface.
func (metadata *Metadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = MetadataFromBytes(data, metadata)

	return
}

// CachedMetadata is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedObjects, without having to manually type cast over and over again.
type CachedMetadata struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedMetadata *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{cachedMetadata.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedMetadata *CachedMetadata) Consume(consumer func(payload *Metadata)) bool {
	return cachedMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Metadata))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedMetadata *CachedMetadata) Unwrap() *Metadata {
	if untypedTransaction := cachedMetadata.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Metadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
