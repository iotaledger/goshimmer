package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// TransactionMetadata contains the information of a Transaction, that are based on our local perception of things (i.e. if it is
// solid, or when we it became solid).
type TransactionMetadata struct {
	objectstorage.StorableObjectFlags

	id                 transaction.Id
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewTransactionMetadata is the constructor for the TransactionMetadata type.
func NewTransactionMetadata(id transaction.Id) *TransactionMetadata {
	return &TransactionMetadata{
		id: id,
	}
}

// TransactionMetadataFromBytes unmarshals a TransactionMetadata object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func TransactionMetadataFromBytes(bytes []byte, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &TransactionMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to TransactionMetadataFromBytes")
	}

	// parse the bytes
	marshalUtil := marshalutil.New(bytes)
	if result.id, err = transaction.ParseId(marshalUtil); err != nil {
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

// TransactionMetadataFromStorage is the factory method for TransactionMetadata objects stored in the objectstorage. The bytes and the content
// will be filled by the objectstorage, by subsequently calling ObjectStorageValue.
func TransactionMetadataFromStorage(storageKey []byte) objectstorage.StorableObject {
	result := &TransactionMetadata{}

	var err error
	if result.id, err = transaction.ParseId(marshalutil.New(storageKey)); err != nil {
		panic(err)
	}

	return result
}

// Parse is a wrapper for simplified unmarshaling of TransactionMetadata objects from a byte stream using the marshalUtil package.
func ParseTransactionMetadata(marshalUtil *marshalutil.MarshalUtil) (*TransactionMetadata, error) {
	if metadata, err := marshalUtil.Parse(func(data []byte) (interface{}, error, int) { return TransactionMetadataFromBytes(data) }); err != nil {
		return nil, err
	} else {
		return metadata.(*TransactionMetadata), nil
	}
}

// Id return the id of the Transaction that this TransactionMetadata is associated to.
func (transactionMetadata *TransactionMetadata) Id() transaction.Id {
	return transactionMetadata.id
}

// Solid returns true if the Transaction has been marked as solid.
func (transactionMetadata *TransactionMetadata) Solid() (result bool) {
	transactionMetadata.solidMutex.RLock()
	result = transactionMetadata.solid
	transactionMetadata.solidMutex.RUnlock()

	return
}

// SetSolid marks a Transaction as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (transactionMetadata *TransactionMetadata) SetSolid(solid bool) (modified bool) {
	transactionMetadata.solidMutex.RLock()
	if transactionMetadata.solid != solid {
		transactionMetadata.solidMutex.RUnlock()

		transactionMetadata.solidMutex.Lock()
		if transactionMetadata.solid != solid {
			transactionMetadata.solid = solid
			if solid {
				transactionMetadata.solidificationTimeMutex.Lock()
				transactionMetadata.solidificationTime = time.Now()
				transactionMetadata.solidificationTimeMutex.Unlock()
			}

			transactionMetadata.SetModified()

			modified = true
		}
		transactionMetadata.solidMutex.Unlock()

	} else {
		transactionMetadata.solidMutex.RUnlock()
	}

	return
}

// SoldificationTime returns the time when the Transaction was marked to be solid.
func (transactionMetadata *TransactionMetadata) SoldificationTime() time.Time {
	transactionMetadata.solidificationTimeMutex.RLock()
	defer transactionMetadata.solidificationTimeMutex.RUnlock()

	return transactionMetadata.solidificationTime
}

// Bytes marshals the TransactionMetadata object into a sequence of bytes.
func (transactionMetadata *TransactionMetadata) Bytes() []byte {
	marshalUtil := marshalutil.New()

	marshalUtil.WriteBytes(transactionMetadata.id.Bytes())
	marshalUtil.WriteTime(transactionMetadata.solidificationTime)
	marshalUtil.WriteBool(transactionMetadata.solid)

	return marshalUtil.Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (transactionMetadata *TransactionMetadata) String() string {
	return stringify.Struct("transaction.TransactionMetadata",
		stringify.StructField("payloadId", transactionMetadata.Id()),
		stringify.StructField("solid", transactionMetadata.Solid()),
		stringify.StructField("solidificationTime", transactionMetadata.SoldificationTime()),
	)
}

// ObjectStorageKey returns the key that is used to identify the TransactionMetadata in the objectstorage.
func (transactionMetadata *TransactionMetadata) ObjectStorageKey() []byte {
	return transactionMetadata.id.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (transactionMetadata *TransactionMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// ObjectStorageValue marshals the TransactionMetadata object into a sequence of bytes and matches the encoding.BinaryMarshaler
// interface.
func (transactionMetadata *TransactionMetadata) ObjectStorageValue() []byte {
	return transactionMetadata.Bytes()
}

// UnmarshalObjectStorageValue restores the values of a TransactionMetadata object from a sequence of bytes and matches the
// encoding.BinaryUnmarshaler interface.
func (transactionMetadata *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (err error) {
	_, err, _ = TransactionMetadataFromBytes(data, transactionMetadata)

	return
}

// CachedTransactionMetadata is a wrapper for the object storage, that takes care of type casting the TransactionMetadata objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of TransactionMetadata, without having to manually type cast over and over again.
type CachedTransactionMetadata struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedTransactionMetadata instead of a generic CachedObject.
func (cachedTransactionMetadata *CachedTransactionMetadata) Retain() *CachedTransactionMetadata {
	return &CachedTransactionMetadata{cachedTransactionMetadata.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedTransactionMetadata object instead of a generic CachedObject in the
// consumer).
func (cachedTransactionMetadata *CachedTransactionMetadata) Consume(consumer func(metadata *TransactionMetadata)) bool {
	return cachedTransactionMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TransactionMetadata))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
func (cachedTransactionMetadata *CachedTransactionMetadata) Unwrap() *TransactionMetadata {
	if untypedTransaction := cachedTransactionMetadata.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*TransactionMetadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &TransactionMetadata{}
