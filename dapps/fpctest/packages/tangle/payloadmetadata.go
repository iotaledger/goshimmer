package tangle

import (
	"sync"

	"github.com/iotaledger/goshimmer/dapps/fpctest/packages/payload"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

// PayloadMetadata is a container for the metadata of a value transfer payload.
// It is used to store the information in the database.
type PayloadMetadata struct {
	objectstorage.StorableObjectFlags

	payloadID payload.ID
	liked     bool

	likedMutex sync.RWMutex
}

// NewPayloadMetadata creates an empty container for the metadata of a value transfer payload.
func NewPayloadMetadata(payloadID payload.ID) *PayloadMetadata {
	return &PayloadMetadata{
		payloadID: payloadID,
	}
}

// PayloadMetadataFromBytes unmarshals a container with the metadata of a value transfer payload from a sequence of bytes.
// It either creates a new container or fills the optionally provided container with the parsed information.
func PayloadMetadataFromBytes(bytes []byte, optionalTargetObject ...*PayloadMetadata) (result *PayloadMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParsePayloadMetadata(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParsePayloadMetadata is a wrapper for simplified unmarshaling in a byte stream using the marshalUtil package.
func ParsePayloadMetadata(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*PayloadMetadata) (result *PayloadMetadata, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return PayloadMetadataFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*PayloadMetadata)
	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// PayloadMetadataFromStorageKey gets called when we restore transaction metadata from the storage. The bytes and the content will be
// unmarshaled by an external caller using the binary.ObjectStorageValue interface.
func PayloadMetadataFromStorageKey(id []byte, optionalTargetObject ...*PayloadMetadata) (result *PayloadMetadata, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &PayloadMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to PayloadMetadataFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(id)
	if result.payloadID, err = payload.ParseID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// PayloadID return the id of the payload that this metadata is associated to.
func (payloadMetadata *PayloadMetadata) PayloadID() payload.ID {
	return payloadMetadata.payloadID
}

// IsLiked returns true if the payload has been marked as liked.
func (payloadMetadata *PayloadMetadata) IsLiked() (result bool) {
	payloadMetadata.likedMutex.RLock()
	result = payloadMetadata.liked
	payloadMetadata.likedMutex.RUnlock()

	return
}

// SetLike sets a payloadMetadata as either liked or disliked.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (payloadMetadata *PayloadMetadata) SetLike(liked bool) (modified bool) {
	payloadMetadata.likedMutex.RLock()
	if payloadMetadata.liked != liked {
		payloadMetadata.likedMutex.RUnlock()

		payloadMetadata.likedMutex.Lock()
		if payloadMetadata.liked != liked {
			payloadMetadata.liked = liked
			// if solid {
			// 	payloadMetadata.solidificationTimeMutex.Lock()
			// 	payloadMetadata.solidificationTime = time.Now()
			// 	payloadMetadata.solidificationTimeMutex.Unlock()
			// }

			payloadMetadata.SetModified()

			modified = true
		}
		payloadMetadata.likedMutex.Unlock()

	} else {
		payloadMetadata.likedMutex.RUnlock()
	}

	return
}

// // SoldificationTime returns the time when the payload was marked to be solid.
// func (payloadMetadata *PayloadMetadata) SoldificationTime() time.Time {
// 	payloadMetadata.solidificationTimeMutex.RLock()
// 	defer payloadMetadata.solidificationTimeMutex.RUnlock()

// 	return payloadMetadata.solidificationTime
// }

// Bytes marshals the metadata into a sequence of bytes.
func (payloadMetadata *PayloadMetadata) Bytes() []byte {
	return marshalutil.New(payload.IDLength + marshalutil.BOOL_SIZE).
		WriteBytes(payloadMetadata.ObjectStorageKey()).
		WriteBytes(payloadMetadata.ObjectStorageValue()).
		Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (payloadMetadata *PayloadMetadata) String() string {
	return stringify.Struct("PayloadMetadata",
		stringify.StructField("payloadId", payloadMetadata.PayloadID()),
		stringify.StructField("liked", payloadMetadata.IsLiked()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database.
// It is required to match StorableObject interface.
func (payloadMetadata *PayloadMetadata) ObjectStorageKey() []byte {
	return payloadMetadata.payloadID.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
// It is required to match StorableObject interface.
func (payloadMetadata *PayloadMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// ObjectStorageValue is required to match the encoding.BinaryMarshaler interface.
func (payloadMetadata *PayloadMetadata) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.BOOL_SIZE).
		WriteBool(payloadMetadata.liked).
		Bytes()
}

// UnmarshalObjectStorageValue is required to match the encoding.BinaryUnmarshaler interface.
func (payloadMetadata *PayloadMetadata) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if payloadMetadata.liked, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// CachedPayloadMetadata is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedObjects, without having to manually type cast over and over again.
type CachedPayloadMetadata struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedPayloadMetadata *CachedPayloadMetadata) Retain() *CachedPayloadMetadata {
	return &CachedPayloadMetadata{cachedPayloadMetadata.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedPayloadMetadata *CachedPayloadMetadata) Consume(consumer func(payload *PayloadMetadata)) bool {
	return cachedPayloadMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*PayloadMetadata))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedPayloadMetadata *CachedPayloadMetadata) Unwrap() *PayloadMetadata {
	untypedTransaction := cachedPayloadMetadata.Get()
	if untypedTransaction == nil {
		return nil
	}

	typeCastedTransaction := untypedTransaction.(*PayloadMetadata)
	if typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
		return nil
	}

	return typeCastedTransaction
}
