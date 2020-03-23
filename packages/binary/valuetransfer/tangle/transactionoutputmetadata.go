package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/marshalutil"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// TransactionOutputMetadata contains the information of a transaction output, that are based on our local perception of things (i.e. if it
// is solid, or when we it became solid).
type TransactionOutputMetadata struct {
	objectstorage.StorableObjectFlags

	id                 transaction.OutputId
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewOutputMetadata is the constructor for the TransactionOutputMetadata type.
func NewTransactionOutputMetadata(outputId transaction.OutputId) *TransactionOutputMetadata {
	return &TransactionOutputMetadata{
		id: outputId,
	}
}

// TransactionOutputMetadataFromBytes unmarshals a TransactionOutputMetadata object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func TransactionOutputMetadataFromBytes(bytes []byte, optionalTargetObject ...*TransactionOutputMetadata) (result *TransactionOutputMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &TransactionOutputMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to TransactionOutputMetadataFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.id, err = transaction.ParseOutputId(marshalUtil); err != nil {
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

// TransactionOutputMetadataFromStorage is the factory method for TransactionOutputMetadata objects stored in the objectstorage. The bytes and the content
// will be filled by the objectstorage, by subsequently calling MarshalBinary.
func TransactionOutputMetadataFromStorage(storageKey []byte) objectstorage.StorableObject {
	result := &TransactionOutputMetadata{}

	var err error
	if result.id, err = transaction.ParseOutputId(marshalutil.New(storageKey)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling of TransactionOutputMetadata objects from a byte stream using the marshalUtil package.
func ParseTransactionOutputMetadata(marshalUtil *marshalutil.MarshalUtil) (*TransactionOutputMetadata, error) {
	if outputMetadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return TransactionOutputMetadataFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return outputMetadata.(*TransactionOutputMetadata), nil
	}
}

// OutputId returns the id of the Output that this TransactionOutputMetadata is associated to.
func (transactionOutputMetadata *TransactionOutputMetadata) Id() transaction.OutputId {
	return transactionOutputMetadata.id
}

// Solid returns true if the Output has been marked as solid.
func (transactionOutputMetadata *TransactionOutputMetadata) Solid() (result bool) {
	transactionOutputMetadata.solidMutex.RLock()
	result = transactionOutputMetadata.solid
	transactionOutputMetadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a Output as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (transactionOutputMetadata *TransactionOutputMetadata) SetSolid(solid bool) (modified bool) {
	transactionOutputMetadata.solidMutex.RLock()
	if transactionOutputMetadata.solid != solid {
		transactionOutputMetadata.solidMutex.RUnlock()

		transactionOutputMetadata.solidMutex.Lock()
		if transactionOutputMetadata.solid != solid {
			transactionOutputMetadata.solid = solid
			if solid {
				transactionOutputMetadata.solidificationTimeMutex.Lock()
				transactionOutputMetadata.solidificationTime = time.Now()
				transactionOutputMetadata.solidificationTimeMutex.Unlock()
			}

			transactionOutputMetadata.SetModified()

			modified = true
		}
		transactionOutputMetadata.solidMutex.Unlock()

	} else {
		transactionOutputMetadata.solidMutex.RUnlock()
	}

	return
}

// SoldificationTime returns the time when the Output was marked to be solid.
func (transactionOutputMetadata *TransactionOutputMetadata) SoldificationTime() time.Time {
	transactionOutputMetadata.solidificationTimeMutex.RLock()
	defer transactionOutputMetadata.solidificationTimeMutex.RUnlock()

	return transactionOutputMetadata.solidificationTime
}

// Bytes marshals the TransactionOutputMetadata object into a sequence of bytes.
func (transactionOutputMetadata *TransactionOutputMetadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(transactionOutputMetadata.id.Bytes())
	marshalUtil.WriteTime(transactionOutputMetadata.solidificationTime)
	marshalUtil.WriteBool(transactionOutputMetadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (transactionOutputMetadata *TransactionOutputMetadata) String() string {
	return stringify.Struct("transaction.TransactionOutputMetadata",
		stringify.StructField("payloadId", transactionOutputMetadata.Id()),
		stringify.StructField("solid", transactionOutputMetadata.Solid()),
		stringify.StructField("solidificationTime", transactionOutputMetadata.SoldificationTime()),
	)
}

// GetStorageKey returns the key that is used to identify the TransactionOutputMetadata in the objectstorage.
func (transactionOutputMetadata *TransactionOutputMetadata) GetStorageKey() []byte {
	return transactionOutputMetadata.id.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (transactionOutputMetadata *TransactionOutputMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// MarshalBinary marshals the TransactionOutputMetadata object into a sequence of bytes and matches the encoding.BinaryMarshaler
// interface.
func (transactionOutputMetadata *TransactionOutputMetadata) MarshalBinary() ([]byte, error) {
	return transactionOutputMetadata.Bytes(), nil
}

// UnmarshalBinary restores the values of a TransactionOutputMetadata object from a sequence of bytes and matches the
// encoding.BinaryUnmarshaler interface.
func (transactionOutputMetadata *TransactionOutputMetadata) UnmarshalBinary(data []byte) (err error) {
	_, err, _ = TransactionOutputMetadataFromBytes(data, transactionOutputMetadata)

	return
}

// CachedTransactionOutputMetadata is a wrapper for the object storage, that takes care of type casting the TransactionOutputMetadata objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of TransactionOutputMetadata, without having to manually type cast over and over again.
type CachedTransactionOutputMetadata struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedTransactionOutputMetadata instead of a generic CachedObject.
func (cachedOutputMetadata *CachedTransactionOutputMetadata) Retain() *CachedTransactionOutputMetadata {
	return &CachedTransactionOutputMetadata{cachedOutputMetadata.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedTransactionOutputMetadata object instead of a generic CachedObject in the
// consumer).
func (cachedOutputMetadata *CachedTransactionOutputMetadata) Consume(consumer func(outputMetadata *TransactionOutputMetadata)) bool {
	return cachedOutputMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TransactionOutputMetadata))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
func (cachedOutputMetadata *CachedTransactionOutputMetadata) Unwrap() *TransactionOutputMetadata {
	if untypedTransaction := cachedOutputMetadata.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*TransactionOutputMetadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
