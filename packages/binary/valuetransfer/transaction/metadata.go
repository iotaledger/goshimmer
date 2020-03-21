package transaction

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

// Metadata contains the information of a Transaction, that are based on our local perception of things (i.e. if it is
// solid, or when we it became solid).
type Metadata struct {
	objectstorage.StorableObjectFlags

	id                 Id
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewMetadata is the constructor for the Metadata type.
func NewMetadata(id Id) *Metadata {
	return &Metadata{
		id: id,
	}
}

// MetadataFromBytes unmarshals a Metadata object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
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
	if result.id, err = ParseId(marshalUtil); err != nil {
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

// MetadataFromStorage is the factory method for Metadata objects stored in the objectstorage. The bytes and the content
// will be filled by the objectstorage, by subsequently calling MarshalBinary.
func MetadataFromStorage(storageKey []byte) objectstorage.StorableObject {
	result := &Metadata{}

	var err error
	if result.id, err = ParseId(marshalutil.New(storageKey)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling of Metadata objects from a byte stream using the marshalUtil package.
func ParseMetadata(marshalUtil *marshalutil.MarshalUtil) (*Metadata, error) {
	if metadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return MetadataFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return metadata.(*Metadata), nil
	}
}

// Id return the id of the Transaction that this Metadata is associated to.
func (metadata *Metadata) Id() Id {
	return metadata.id
}

// Solid returns true if the Transaction has been marked as solid.
func (metadata *Metadata) Solid() (result bool) {
	metadata.solidMutex.RLock()
	result = metadata.solid
	metadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a Transaction as either solid or not solid.
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

// SoldificationTime returns the time when the Transaction was marked to be solid.
func (metadata *Metadata) SoldificationTime() time.Time {
	metadata.solidificationTimeMutex.RLock()
	defer metadata.solidificationTimeMutex.RUnlock()

	return metadata.solidificationTime
}

// Bytes marshals the Metadata object into a sequence of bytes.
func (metadata *Metadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(metadata.id.Bytes())
	marshalUtil.WriteTime(metadata.solidificationTime)
	marshalUtil.WriteBool(metadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (metadata *Metadata) String() string {
	return stringify.Struct("transaction.Metadata",
		stringify.StructField("payloadId", metadata.Id()),
		stringify.StructField("solid", metadata.Solid()),
		stringify.StructField("solidificationTime", metadata.SoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to identify the Metadata in the objectstorage.
func (metadata *Metadata) GetStorageKey() []byte {
	return metadata.id.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (metadata *Metadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary marshals the Metadata object into a sequence of bytes and matches the encoding.BinaryMarshaler
// interface.
func (metadata *Metadata) MarshalBinary() ([]byte, error) {
	return metadata.Bytes(), nil
}

// UnmarshalBinary restores the values of a Metadata object from a sequence of bytes and matches the
// encoding.BinaryUnmarshaler interface.
func (metadata *Metadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = MetadataFromBytes(data, metadata)

	return
}

// CachedMetadata is a wrapper for the object storage, that takes care of type casting the Metadata objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of Metadata, without having to manually type cast over and over again.
type CachedMetadata struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedMetadata instead of a generic CachedObject.
func (cachedMetadata *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{cachedMetadata.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedMetadata object instead of a generic CachedObject in the
// consumer).
func (cachedMetadata *CachedMetadata) Consume(consumer func(metadata *Metadata)) bool {
	return cachedMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Metadata))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
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
