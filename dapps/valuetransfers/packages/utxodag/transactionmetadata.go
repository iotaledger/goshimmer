package utxodag

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// TransactionMetadata contains the information of a Transaction, that are based on our local perception of things (i.e. if it is
// solid, or when we it became solid).
type TransactionMetadata struct {
	objectstorage.StorableObjectFlags

	id                 transaction.Id
	branchId           branchmanager.BranchId
	solid              bool
	finalized          bool
	solidificationTime time.Time
	finalizationTime   time.Time

	branchIdMutex           sync.RWMutex
	solidMutex              sync.RWMutex
	finalizedMutex          sync.RWMutex
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
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseTransactionMetadata(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionMetadataFromStorageKey get's called when we restore TransactionMetadata from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func TransactionMetadataFromStorageKey(keyBytes []byte, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &TransactionMetadata{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to TransactionMetadataFromStorageKey")
	}

	// parse information
	marshalUtil := marshalutil.New(keyBytes)
	result.id, err = transaction.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Parse is a wrapper for simplified unmarshaling of TransactionMetadata objects from a byte stream using the marshalUtil package.
func ParseTransactionMetadata(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return TransactionMetadataFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*TransactionMetadata)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// Id return the id of the Transaction that this TransactionMetadata is associated to.
func (transactionMetadata *TransactionMetadata) Id() transaction.Id {
	return transactionMetadata.id
}

func (transactionMetadata *TransactionMetadata) BranchId() branchmanager.BranchId {
	transactionMetadata.branchIdMutex.RLock()
	defer transactionMetadata.branchIdMutex.RUnlock()

	return transactionMetadata.branchId
}

func (transactionMetadata *TransactionMetadata) SetBranchId(branchId branchmanager.BranchId) (modified bool) {
	transactionMetadata.branchIdMutex.RLock()
	if transactionMetadata.branchId == branchId {
		transactionMetadata.branchIdMutex.RUnlock()

		return
	}

	transactionMetadata.branchIdMutex.RUnlock()
	transactionMetadata.branchIdMutex.Lock()
	defer transactionMetadata.branchIdMutex.Unlock()

	if transactionMetadata.branchId == branchId {
		return
	}

	transactionMetadata.branchId = branchId
	modified = true

	return
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

func (transactionMetadata *TransactionMetadata) SetFinalized(finalized bool) (modified bool) {
	transactionMetadata.finalizedMutex.RLock()
	if transactionMetadata.finalized == finalized {
		transactionMetadata.finalizedMutex.RUnlock()

		return
	}

	transactionMetadata.finalizedMutex.RUnlock()
	transactionMetadata.finalizedMutex.Lock()
	defer transactionMetadata.finalizedMutex.Unlock()

	if transactionMetadata.finalized == finalized {
		return
	}

	transactionMetadata.finalized = finalized
	if finalized {
		transactionMetadata.finalizationTime = time.Now()
	}
	modified = true

	return
}

func (transactionMetadata *TransactionMetadata) Finalized() bool {
	transactionMetadata.finalizedMutex.RLock()
	defer transactionMetadata.finalizedMutex.RUnlock()

	return transactionMetadata.finalized
}

func (transactionMetadata *TransactionMetadata) FinalizationTime() time.Time {
	transactionMetadata.finalizedMutex.RLock()
	defer transactionMetadata.finalizedMutex.RUnlock()

	return transactionMetadata.finalizationTime
}

// SoldificationTime returns the time when the Transaction was marked to be solid.
func (transactionMetadata *TransactionMetadata) SoldificationTime() time.Time {
	transactionMetadata.solidificationTimeMutex.RLock()
	defer transactionMetadata.solidificationTimeMutex.RUnlock()

	return transactionMetadata.solidificationTime
}

// Bytes marshals the TransactionMetadata object into a sequence of bytes.
func (transactionMetadata *TransactionMetadata) Bytes() []byte {
	return marshalutil.New(branchmanager.BranchIdLength + 2*marshalutil.TIME_SIZE + 2*marshalutil.BOOL_SIZE).
		WriteBytes(transactionMetadata.BranchId().Bytes()).
		WriteTime(transactionMetadata.solidificationTime).
		WriteTime(transactionMetadata.finalizationTime).
		WriteBool(transactionMetadata.solid).
		WriteBool(transactionMetadata.finalized).
		Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (transactionMetadata *TransactionMetadata) String() string {
	return stringify.Struct("transaction.TransactionMetadata",
		stringify.StructField("id", transactionMetadata.Id()),
		stringify.StructField("branchId", transactionMetadata.BranchId()),
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
func (transactionMetadata *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(data)
	if transactionMetadata.branchId, err = branchmanager.ParseBranchId(marshalUtil); err != nil {
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if transactionMetadata.finalizationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if transactionMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

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
