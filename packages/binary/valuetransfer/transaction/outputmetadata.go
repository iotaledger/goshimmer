package transaction

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
)

// OutputMetadata contains the information of a transfer output, that are based on our local perception of things (i.e. if it
// is solid, or when we it became solid).
type OutputMetadata struct {
	objectstorage.StorableObjectFlags

	id                 OutputId
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewOutputMetadata is the constructor for the OutputMetadata type.
func NewOutputMetadata(outputId OutputId) *OutputMetadata {
	return &OutputMetadata{
		id: outputId,
	}
}

// OutputMetadataFromBytes unmarshals a OutputMetadata object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func OutputMetadataFromBytes(bytes []byte, optionalTargetObject ...*OutputMetadata) (result *OutputMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &OutputMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to OutputMetadataFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.id, err = ParseOutputId(marshalUtil); err != nil {
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

// OutputMetadataFromStorage is the factory method for OutputMetadata objects stored in the objectstorage. The bytes and the content
// will be filled by the objectstorage, by subsequently calling MarshalBinary.
func OutputMetadataFromStorage(storageKey []byte) objectstorage.StorableObject {
	result := &OutputMetadata{}

	var err error
	if result.id, err = ParseOutputId(marshalutil.New(storageKey)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling of OutputMetadata objects from a byte stream using the marshalUtil package.
func ParseOutputMetadata(marshalUtil *marshalutil.MarshalUtil) (*OutputMetadata, error) {
	if outputMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return OutputMetadataFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return outputMetadata.(*OutputMetadata), nil
	}
}

// OutputId returns the id of the Output that this OutputMetadata is associated to.
func (outputMetadata *OutputMetadata) Id() OutputId {
	return outputMetadata.id
}

// Solid returns true if the Output has been marked as solid.
func (outputMetadata *OutputMetadata) Solid() (result bool) {
	outputMetadata.solidMutex.RLock()
	result = outputMetadata.solid
	outputMetadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a Output as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (outputMetadata *OutputMetadata) SetSolid(solid bool) (modified bool) {
	outputMetadata.solidMutex.RLock()
	if outputMetadata.solid != solid {
		outputMetadata.solidMutex.RUnlock()

		outputMetadata.solidMutex.Lock()
		if outputMetadata.solid != solid {
			outputMetadata.solid = solid
			if solid {
				outputMetadata.solidificationTimeMutex.Lock()
				outputMetadata.solidificationTime = time.Now()
				outputMetadata.solidificationTimeMutex.Unlock()
			}

			outputMetadata.SetModified()

			modified = true
		}
		outputMetadata.solidMutex.Unlock()

	} else {
		outputMetadata.solidMutex.RUnlock()
	}

	return
}

// SoldificationTime returns the time when the Output was marked to be solid.
func (outputMetadata *OutputMetadata) SoldificationTime() time.Time {
	outputMetadata.solidificationTimeMutex.RLock()
	defer outputMetadata.solidificationTimeMutex.RUnlock()

	return outputMetadata.solidificationTime
}

// Bytes marshals the OutputMetadata object into a sequence of bytes.
func (outputMetadata *OutputMetadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(outputMetadata.id.Bytes())
	marshalUtil.WriteTime(outputMetadata.solidificationTime)
	marshalUtil.WriteBool(outputMetadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (outputMetadata *OutputMetadata) String() string {
	return stringify.Struct("transaction.OutputMetadata",
		stringify.StructField("payloadId", outputMetadata.Id()),
		stringify.StructField("solid", outputMetadata.Solid()),
		stringify.StructField("solidificationTime", outputMetadata.SoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to identify the OutputMetadata in the objectstorage.
func (outputMetadata *OutputMetadata) GetStorageKey() []byte {
	return outputMetadata.id.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (outputMetadata *OutputMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary marshals the OutputMetadata object into a sequence of bytes and matches the encoding.BinaryMarshaler
// interface.
func (outputMetadata *OutputMetadata) MarshalBinary() ([]byte, error) {
	return outputMetadata.Bytes(), nil
}

// UnmarshalBinary restores the values of a OutputMetadata object from a sequence of bytes and matches the
// encoding.BinaryUnmarshaler interface.
func (outputMetadata *OutputMetadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = OutputMetadataFromBytes(data, outputMetadata)

	return
}

// CachedOutputMetadata is a wrapper for the object storage, that takes care of type casting the OutputMetadata objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of OutputMetadata, without having to manually type cast over and over again.
type CachedOutputMetadata struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedOutputMetadata instead of a generic CachedObject.
func (cachedOutputMetadata *CachedOutputMetadata) Retain() *CachedOutputMetadata {
	return &CachedOutputMetadata{cachedOutputMetadata.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedOutputMetadata object instead of a generic CachedObject in the
// consumer).
func (cachedOutputMetadata *CachedOutputMetadata) Consume(consumer func(outputMetadata *OutputMetadata)) bool {
	return cachedOutputMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*OutputMetadata))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
func (cachedOutputMetadata *CachedOutputMetadata) Unwrap() *OutputMetadata {
	if untypedTransaction := cachedOutputMetadata.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*OutputMetadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
