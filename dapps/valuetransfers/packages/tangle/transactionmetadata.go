package tangle

import (
	"sync"
	"time"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
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
func (output *TransactionMetadata) ID() transaction.ID {
	return output.id
}

// BranchID returns the identifier of the Branch, that this transaction is booked into.
func (output *TransactionMetadata) BranchID() branchmanager.BranchID {
	output.branchIDMutex.RLock()
	defer output.branchIDMutex.RUnlock()

	return output.branchID
}

// SetBranchID is the setter for the branch id. It returns true if the value of the flag has been updated.
func (output *TransactionMetadata) SetBranchID(branchID branchmanager.BranchID) (modified bool) {
	output.branchIDMutex.RLock()
	if output.branchID == branchID {
		output.branchIDMutex.RUnlock()

		return
	}

	output.branchIDMutex.RUnlock()
	output.branchIDMutex.Lock()
	defer output.branchIDMutex.Unlock()

	if output.branchID == branchID {
		return
	}

	output.branchID = branchID
	modified = true

	return
}

// Conflicting returns true if the Transaction has been forked into its own Branch and there is a vote going on.
func (output *TransactionMetadata) Conflicting() bool {
	return output.BranchID() == branchmanager.NewBranchID(output.ID())
}

// Solid returns true if the Transaction has been marked as solid.
func (output *TransactionMetadata) Solid() (result bool) {
	output.solidMutex.RLock()
	result = output.solid
	output.solidMutex.RUnlock()

	return
}

// SetSolid marks a Transaction as either solid or not solid.
// It returns true if the solid flag was changes and automatically updates the solidificationTime as well.
func (output *TransactionMetadata) SetSolid(solid bool) (modified bool) {
	output.solidMutex.RLock()
	if output.solid != solid {
		output.solidMutex.RUnlock()

		output.solidMutex.Lock()
		if output.solid != solid {
			output.solid = solid
			if solid {
				output.solidificationTimeMutex.Lock()
				output.solidificationTime = time.Now()
				output.solidificationTimeMutex.Unlock()
			}

			output.SetModified()

			modified = true
		}
		output.solidMutex.Unlock()

	} else {
		output.solidMutex.RUnlock()
	}

	return
}

// Preferred returns true if the transaction is considered to be the first valid spender of all of its Inputs.
func (output *TransactionMetadata) Preferred() (result bool) {
	output.preferredMutex.RLock()
	defer output.preferredMutex.RUnlock()

	return output.preferred
}

// setPreferred updates the preferred flag of the transaction. It is defined as a private setter because updating the
// preferred flag causes changes in other transactions and branches as well. This means that we need additional logic
// in the tangle. To update the preferred flag of a transaction, we need to use Tangle.SetTransactionPreferred(bool).
func (output *TransactionMetadata) setPreferred(preferred bool) (modified bool) {
	output.preferredMutex.RLock()
	if output.preferred == preferred {
		output.preferredMutex.RUnlock()

		return
	}

	output.preferredMutex.RUnlock()
	output.preferredMutex.Lock()
	defer output.preferredMutex.Unlock()

	if output.preferred == preferred {
		return
	}

	output.preferred = preferred
	output.SetModified()
	modified = true

	return
}

// SetFinalized allows us to set the finalized flag on the transactions. Finalized transactions will not be forked when
// a conflict arrives later.
func (output *TransactionMetadata) SetFinalized(finalized bool) (modified bool) {
	output.finalizedMutex.RLock()
	if output.finalized == finalized {
		output.finalizedMutex.RUnlock()

		return
	}

	output.finalizedMutex.RUnlock()
	output.finalizedMutex.Lock()
	defer output.finalizedMutex.Unlock()

	if output.finalized == finalized {
		return
	}

	output.finalized = finalized
	output.SetModified()
	if finalized {
		output.finalizationTime = time.Now()
	}
	modified = true

	return
}

// Finalized returns true, if the decision if this transaction is liked or not has been finalized by consensus already.
func (output *TransactionMetadata) Finalized() bool {
	output.finalizedMutex.RLock()
	defer output.finalizedMutex.RUnlock()

	return output.finalized
}

// Liked returns true if the Transaction was marked as liked.
func (output *TransactionMetadata) Liked() bool {
	output.likedMutex.RLock()
	defer output.likedMutex.RUnlock()

	return output.liked
}

// setLiked modifies the liked flag of the given Transaction. It returns true if the value has been updated.
func (output *TransactionMetadata) setLiked(liked bool) (modified bool) {
	output.likedMutex.RLock()
	if output.liked == liked {
		output.likedMutex.RUnlock()

		return
	}

	output.likedMutex.RUnlock()
	output.likedMutex.Lock()
	defer output.likedMutex.Unlock()

	if output.liked == liked {
		return
	}

	output.liked = liked
	output.SetModified()
	modified = true

	return
}

// Confirmed returns true if the Transaction was marked as confirmed.
func (output *TransactionMetadata) Confirmed() bool {
	output.confirmedMutex.RLock()
	defer output.confirmedMutex.RUnlock()

	return output.confirmed
}

// setConfirmed modifies the confirmed flag of the given Transaction. It returns true if the value has been updated.
func (output *TransactionMetadata) setConfirmed(confirmed bool) (modified bool) {
	output.confirmedMutex.RLock()
	if output.confirmed == confirmed {
		output.confirmedMutex.RUnlock()

		return
	}

	output.confirmedMutex.RUnlock()
	output.confirmedMutex.Lock()
	defer output.confirmedMutex.Unlock()

	if output.confirmed == confirmed {
		return
	}

	output.confirmed = confirmed
	output.SetModified()
	modified = true

	return
}

// Rejected returns true if the Transaction was marked as confirmed.
func (output *TransactionMetadata) Rejected() bool {
	output.rejectedMutex.RLock()
	defer output.rejectedMutex.RUnlock()

	return output.rejected
}

// setRejected modifies the rejected flag of the given Transaction. It returns true if the value has been updated.
func (output *TransactionMetadata) setRejected(rejected bool) (modified bool) {
	output.rejectedMutex.RLock()
	if output.rejected == rejected {
		output.rejectedMutex.RUnlock()

		return
	}

	output.rejectedMutex.RUnlock()
	output.rejectedMutex.Lock()
	defer output.rejectedMutex.Unlock()

	if output.rejected == rejected {
		return
	}

	output.rejected = rejected
	output.SetModified()
	modified = true

	return
}

// FinalizationTime returns the time when this transaction was finalized.
func (output *TransactionMetadata) FinalizationTime() time.Time {
	output.finalizedMutex.RLock()
	defer output.finalizedMutex.RUnlock()

	return output.finalizationTime
}

// SoldificationTime returns the time when the Transaction was marked to be solid.
func (output *TransactionMetadata) SoldificationTime() time.Time {
	output.solidificationTimeMutex.RLock()
	defer output.solidificationTimeMutex.RUnlock()

	return output.solidificationTime
}

// Bytes marshals the TransactionMetadata object into a sequence of bytes.
func (output *TransactionMetadata) Bytes() []byte {
	return marshalutil.New(branchmanager.BranchIDLength + 2*marshalutil.TIME_SIZE + 6*marshalutil.BOOL_SIZE).
		WriteBytes(output.BranchID().Bytes()).
		WriteTime(output.SoldificationTime()).
		WriteTime(output.FinalizationTime()).
		WriteBool(output.Solid()).
		WriteBool(output.Preferred()).
		WriteBool(output.Finalized()).
		WriteBool(output.Liked()).
		WriteBool(output.Confirmed()).
		WriteBool(output.Rejected()).
		Bytes()
}

// String creates a human readable version of the metadata (for debug purposes).
func (output *TransactionMetadata) String() string {
	return stringify.Struct("transaction.TransactionMetadata",
		stringify.StructField("id", output.ID()),
		stringify.StructField("branchId", output.BranchID()),
		stringify.StructField("solid", output.Solid()),
		stringify.StructField("solidificationTime", output.SoldificationTime()),
	)
}

// ObjectStorageKey returns the key that is used to identify the TransactionMetadata in the objectstorage.
func (output *TransactionMetadata) ObjectStorageKey() []byte {
	return output.id.Bytes()
}

// Update is disabled and panics if it ever gets called - updates are supposed to happen through the setters.
func (output *TransactionMetadata) Update(other objectstorage.StorableObject) {
	panic("update forbidden")
}

// ObjectStorageValue marshals the TransactionMetadata object into a sequence of bytes and matches the encoding.BinaryMarshaler
// interface.
func (output *TransactionMetadata) ObjectStorageValue() []byte {
	return output.Bytes()
}

// UnmarshalObjectStorageValue restores the values of a TransactionMetadata object from a sequence of bytes and matches the
// encoding.BinaryUnmarshaler interface.
func (output *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(data)
	if output.branchID, err = branchmanager.ParseBranchID(marshalUtil); err != nil {
		return
	}
	if output.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if output.finalizationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if output.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.preferred, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.finalized, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.liked, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.confirmed, err = marshalUtil.ReadBool(); err != nil {
		return
	}
	if output.rejected, err = marshalUtil.ReadBool(); err != nil {
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
