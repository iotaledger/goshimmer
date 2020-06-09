package tangle

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

	id                 transaction.ID
	branchID           branchmanager.BranchID
	solid              bool
	preferred          bool
	finalized          bool
	liked              bool
	confirmed          bool
	rejected           bool
	solidificationTime time.Time
	finalizationTime   time.Time

	branchIDMutex           sync.RWMutex
	solidMutex              sync.RWMutex
	preferredMutex          sync.RWMutex
	finalizedMutex          sync.RWMutex
	likedMutex              sync.RWMutex
	confirmedMutex          sync.RWMutex
	rejectedMutex           sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

// NewTransactionMetadata is the constructor for the TransactionMetadata type.
func NewTransactionMetadata(id transaction.ID) *TransactionMetadata {
	return &TransactionMetadata{
		id: id,
	}
}

// TransactionMetadataFromBytes unmarshals a TransactionMetadata object from a sequence of bytes.
// It either creates a new object or fills the optionally provided object with the parsed information.
func TransactionMetadataFromBytes(bytes []byte, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseTransactionMetadata(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// TransactionMetadataFromStorageKey get's called when we restore TransactionMetadata from the storage.
// In contrast to other database models, it unmarshals some information from the key so we simply store the key before
// it gets handed over to UnmarshalObjectStorageValue (by the ObjectStorage).
func TransactionMetadataFromStorageKey(keyBytes []byte, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, consumedBytes int, err error) {
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
	result.id, err = transaction.ParseID(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseTransactionMetadata is a wrapper for simplified unmarshaling of TransactionMetadata objects from a byte stream using the marshalUtil package.
func ParseTransactionMetadata(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*TransactionMetadata) (result *TransactionMetadata, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return TransactionMetadataFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*TransactionMetadata)
	_, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	})

	return
}

// ID return the id of the Transaction that this TransactionMetadata is associated to.
func (transactionMetadata *TransactionMetadata) ID() transaction.ID {
	return transactionMetadata.id
}

// BranchID returns the identifier of the Branch, that this transaction is booked into.
func (transactionMetadata *TransactionMetadata) BranchID() branchmanager.BranchID {
	transactionMetadata.branchIDMutex.RLock()
	defer transactionMetadata.branchIDMutex.RUnlock()

	return transactionMetadata.branchID
}

// SetBranchID is the setter for the branch id. It returns true if the value of the flag has been updated.
func (transactionMetadata *TransactionMetadata) SetBranchID(branchID branchmanager.BranchID) (modified bool) {
	transactionMetadata.branchIDMutex.RLock()
	if transactionMetadata.branchID == branchID {
		transactionMetadata.branchIDMutex.RUnlock()

		return
	}

	transactionMetadata.branchIDMutex.RUnlock()
	transactionMetadata.branchIDMutex.Lock()
	defer transactionMetadata.branchIDMutex.Unlock()

	if transactionMetadata.branchID == branchID {
		return
	}

	transactionMetadata.branchID = branchID
	modified = true

	return
}

// Conflicting returns true if the Transaction has been forked into its own Branch and there is a vote going on.
func (transactionMetadata *TransactionMetadata) Conflicting() bool {
	return transactionMetadata.BranchID() == branchmanager.NewBranchID(transactionMetadata.ID())
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

// Preferred returns true if the transaction is considered to be the first valid spender of all of its Inputs.
func (transactionMetadata *TransactionMetadata) Preferred() (result bool) {
	transactionMetadata.preferredMutex.RLock()
	defer transactionMetadata.preferredMutex.RUnlock()

	return transactionMetadata.preferred
}

// setPreferred updates the preferred flag of the transaction. It is defined as a private setter because updating the
// preferred flag causes changes in other transactions and branches as well. This means that we need additional logic
// in the tangle. To update the preferred flag of a transaction, we need to use Tangle.SetTransactionPreferred(bool).
func (transactionMetadata *TransactionMetadata) setPreferred(preferred bool) (modified bool) {
	transactionMetadata.preferredMutex.RLock()
	if transactionMetadata.preferred == preferred {
		transactionMetadata.preferredMutex.RUnlock()

		return
	}

	transactionMetadata.preferredMutex.RUnlock()
	transactionMetadata.preferredMutex.Lock()
	defer transactionMetadata.preferredMutex.Unlock()

	if transactionMetadata.preferred == preferred {
		return
	}

	transactionMetadata.preferred = preferred
	transactionMetadata.SetModified()
	modified = true

	return
}

// SetFinalized allows us to set the finalized flag on the transactions. Finalized transactions will not be forked when
// a conflict arrives later.
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
	transactionMetadata.SetModified()
	if finalized {
		transactionMetadata.finalizationTime = time.Now()
	}
	modified = true

	return
}

// Finalized returns true, if the decision if this transaction is liked or not has been finalized by consensus already.
func (transactionMetadata *TransactionMetadata) Finalized() bool {
	transactionMetadata.finalizedMutex.RLock()
	defer transactionMetadata.finalizedMutex.RUnlock()

	return transactionMetadata.finalized
}

// Liked returns true if the Transaction was marked as liked.
func (transactionMetadata *TransactionMetadata) Liked() bool {
	transactionMetadata.likedMutex.RLock()
	defer transactionMetadata.likedMutex.RUnlock()

	return transactionMetadata.liked
}

// setLiked modifies the liked flag of the given Transaction. It returns true if the value has been updated.
func (transactionMetadata *TransactionMetadata) setLiked(liked bool) (modified bool) {
	transactionMetadata.likedMutex.RLock()
	if transactionMetadata.liked == liked {
		transactionMetadata.likedMutex.RUnlock()

		return
	}

	transactionMetadata.likedMutex.RUnlock()
	transactionMetadata.likedMutex.Lock()
	defer transactionMetadata.likedMutex.Unlock()

	if transactionMetadata.liked == liked {
		return
	}

	transactionMetadata.liked = liked
	transactionMetadata.SetModified()
	modified = true

	return
}

// Confirmed returns true if the Transaction was marked as confirmed.
func (transactionMetadata *TransactionMetadata) Confirmed() bool {
	transactionMetadata.confirmedMutex.RLock()
	defer transactionMetadata.confirmedMutex.RUnlock()

	return transactionMetadata.confirmed
}

// setConfirmed modifies the confirmed flag of the given Transaction. It returns true if the value has been updated.
func (transactionMetadata *TransactionMetadata) setConfirmed(confirmed bool) (modified bool) {
	transactionMetadata.confirmedMutex.RLock()
	if transactionMetadata.confirmed == confirmed {
		transactionMetadata.confirmedMutex.RUnlock()

		return
	}

	transactionMetadata.confirmedMutex.RUnlock()
	transactionMetadata.confirmedMutex.Lock()
	defer transactionMetadata.confirmedMutex.Unlock()

	if transactionMetadata.confirmed == confirmed {
		return
	}

	transactionMetadata.confirmed = confirmed
	transactionMetadata.SetModified()
	modified = true

	return
}

// Rejected returns true if the Transaction was marked as confirmed.
func (transactionMetadata *TransactionMetadata) Rejected() bool {
	transactionMetadata.rejectedMutex.RLock()
	defer transactionMetadata.rejectedMutex.RUnlock()

	return transactionMetadata.rejected
}

// setRejected modifies the rejected flag of the given Transaction. It returns true if the value has been updated.
func (transactionMetadata *TransactionMetadata) setRejected(rejected bool) (modified bool) {
	transactionMetadata.rejectedMutex.RLock()
	if transactionMetadata.rejected == rejected {
		transactionMetadata.rejectedMutex.RUnlock()

		return
	}

	transactionMetadata.rejectedMutex.RUnlock()
	transactionMetadata.rejectedMutex.Lock()
	defer transactionMetadata.rejectedMutex.Unlock()

	if transactionMetadata.rejected == rejected {
		return
	}

	transactionMetadata.rejected = rejected
	transactionMetadata.SetModified()
	modified = true

	return
}

// FinalizationTime returns the time when this transaction was finalized.
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
	return marshalutil.New(branchmanager.BranchIDLength + 2*marshalutil.TIME_SIZE + 6*marshalutil.BOOL_SIZE).
		WriteBytes(transactionMetadata.BranchID().Bytes()).
		WriteTime(transactionMetadata.SoldificationTime()).
		WriteTime(transactionMetadata.FinalizationTime()).
		WriteBool(transactionMetadata.Solid()).
		WriteBool(transactionMetadata.Preferred()).
		WriteBool(transactionMetadata.Finalized()).
		WriteBool(transactionMetadata.Liked()).
		WriteBool(transactionMetadata.Confirmed()).
		WriteBool(transactionMetadata.Rejected()).
		Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (transactionMetadata *TransactionMetadata) String() string {
	return stringify.Struct("transaction.TransactionMetadata",
		stringify.StructField("id", transactionMetadata.ID()),
		stringify.StructField("branchId", transactionMetadata.BranchID()),
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
func (transactionMetadata *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if transactionMetadata.branchID, err = branchmanager.ParseBranchID(marshalUtil); err != nil {
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
	if transactionMetadata.preferred, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if transactionMetadata.finalized, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if transactionMetadata.liked, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if transactionMetadata.confirmed, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if transactionMetadata.rejected, err = marshalUtil.ReadBool(); err != nil {
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
	untypedTransaction := cachedTransactionMetadata.Get()
	if untypedTransaction == nil {
		return nil
	}

	typeCastedTransaction := untypedTransaction.(*TransactionMetadata)
	if typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
		return nil
	}

	return typeCastedTransaction
}

// Interface contract: make compiler warn if the interface is not implemented correctly.
var _ objectstorage.StorableObject = &TransactionMetadata{}
