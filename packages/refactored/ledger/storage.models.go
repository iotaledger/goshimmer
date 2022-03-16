package ledger

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/old"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region TransactionMetadata //////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata contains additional information about a Transaction that is derived from the local perception of
// a node.
type TransactionMetadata struct {
	id                      utxo.TransactionID
	branchIDs               old.BranchIDs
	branchIDsMutex          sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	lazyBooked              bool
	lazyBookedMutex         sync.RWMutex
	gradeOfFinality         gof.GradeOfFinality
	gradeOfFinalityTime     time.Time
	gradeOfFinalityMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata creates a new empty TransactionMetadata object.
func NewTransactionMetadata(transactionID utxo.TransactionID) (newTransactionMetadata *TransactionMetadata) {
	newTransactionMetadata = &TransactionMetadata{
		id:        transactionID,
		branchIDs: old.NewBranchIDs(),
	}
	newTransactionMetadata.SetModified()
	newTransactionMetadata.Persist()

	return newTransactionMetadata
}

// FromObjectStorage creates an TransactionMetadata from sequences of key and bytes.
func (t *TransactionMetadata) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	transactionMetadata, err := t.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from bytes: %w", err)
		return transactionMetadata, err
	}

	return transactionMetadata, err
}

// FromBytes unmarshals an TransactionMetadata object from a sequence of bytes.
func (t *TransactionMetadata) FromBytes(bytes []byte) (transactionMetadata *TransactionMetadata, err error) {
	marshalUtil := marshalutil.New(bytes)
	if transactionMetadata, err = t.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionMetadata from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals an TransactionMetadata object using a MarshalUtil (for easier unmarshalling).
func (t *TransactionMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (transactionMetadata *TransactionMetadata, err error) {
	if transactionMetadata = t; transactionMetadata == nil {
		transactionMetadata = &TransactionMetadata{}
	}
	if transactionMetadata.id, err = utxo.TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID: %w", err)
		return
	}
	if transactionMetadata.branchIDs, err = old.BranchIDsFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse BranchID: %w", err)
		return
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse solidify time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	if transactionMetadata.lazyBooked, err = marshalUtil.ReadBool(); err != nil {
		err = errors.Errorf("failed to parse lazy booked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	transactionMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if transactionMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ID returns the TransactionID of the Transaction that the TransactionMetadata belongs to.
func (t *TransactionMetadata) ID() utxo.TransactionID {
	return t.id
}

// BranchIDs returns the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) BranchIDs() old.BranchIDs {
	t.branchIDsMutex.RLock()
	defer t.branchIDsMutex.RUnlock()

	return t.branchIDs.Clone()
}

// SetBranchIDs sets the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) SetBranchIDs(branchIDs old.BranchIDs) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Equals(branchIDs) {
		return false
	}

	t.branchIDs = branchIDs.Clone()
	t.SetModified()
	return true
}

// AddBranchID adds an identifier of the Branch that the Transaction was booked in.
func (t *TransactionMetadata) AddBranchID(branchID old.BranchID) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Contains(branchID) {
		return false
	}

	delete(t.branchIDs, old.MasterBranchID)

	t.branchIDs.Add(branchID)
	t.SetModified()
	return true
}

// Solid returns true if the Transaction has been marked as solid.
func (t *TransactionMetadata) Solid() bool {
	t.solidMutex.RLock()
	defer t.solidMutex.RUnlock()

	return t.solid
}

// SetSolid updates the solid flag of the Transaction. It returns true if the solid flag was modified and updates the
// solidify time if the Transaction was marked as solid.
func (t *TransactionMetadata) SetSolid(solid bool) (modified bool) {
	t.solidMutex.Lock()
	defer t.solidMutex.Unlock()

	if t.solid == solid {
		return
	}

	if solid {
		t.solidificationTimeMutex.Lock()
		t.solidificationTime = time.Now()
		t.solidificationTimeMutex.Unlock()
	}

	t.solid = solid
	t.SetModified()
	modified = true

	return
}

// SolidificationTime returns the time when the Transaction was marked as solid.
func (t *TransactionMetadata) SolidificationTime() time.Time {
	t.solidificationTimeMutex.RLock()
	defer t.solidificationTimeMutex.RUnlock()

	return t.solidificationTime
}

// LazyBooked returns a boolean flag that indicates if the Transaction has been analyzed regarding the conflicting
// status of its consumed Branches.
func (t *TransactionMetadata) LazyBooked() (lazyBooked bool) {
	t.lazyBookedMutex.RLock()
	defer t.lazyBookedMutex.RUnlock()

	return t.lazyBooked
}

// SetLazyBooked updates the lazy booked flag of the Output. It returns true if the value was modified.
func (t *TransactionMetadata) SetLazyBooked(lazyBooked bool) (modified bool) {
	t.lazyBookedMutex.Lock()
	defer t.lazyBookedMutex.Unlock()

	if t.lazyBooked == lazyBooked {
		return
	}

	t.lazyBooked = lazyBooked
	t.SetModified()
	modified = true

	return
}

// GradeOfFinality returns the grade of finality.
func (t *TransactionMetadata) GradeOfFinality() gof.GradeOfFinality {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()
	return t.gradeOfFinality
}

// SetGradeOfFinality updates the grade of finality. It returns true if it was modified.
func (t *TransactionMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	t.gradeOfFinalityMutex.Lock()
	defer t.gradeOfFinalityMutex.Unlock()

	if t.gradeOfFinality == gradeOfFinality {
		return
	}

	t.gradeOfFinality = gradeOfFinality
	t.gradeOfFinalityTime = clock.SyncedTime()
	t.SetModified()
	modified = true
	return
}

// GradeOfFinalityTime returns the time when the Transaction's gradeOfFinality was set.
func (t *TransactionMetadata) GradeOfFinalityTime() time.Time {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()

	return t.gradeOfFinalityTime
}

// IsConflicting returns true if the Transaction is conflicting with another Transaction (has its own Branch).
func (t *TransactionMetadata) IsConflicting() bool {
	branchIDs := t.BranchIDs()
	return len(branchIDs) == 1 && branchIDs.Contains(old.NewBranchID(t.ID()))
}

// Bytes marshals the TransactionMetadata into a sequence of bytes.
func (t *TransactionMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human-readable version of the TransactionMetadata.
func (t *TransactionMetadata) String() string {
	return stringify.Struct("TransactionMetadata",
		stringify.StructField("id", t.ID()),
		stringify.StructField("branchID", t.BranchIDs()),
		stringify.StructField("solid", t.Solid()),
		stringify.StructField("solidificationTime", t.SolidificationTime()),
		stringify.StructField("lazyBooked", t.LazyBooked()),
		stringify.StructField("gradeOfFinality", t.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", t.GradeOfFinalityTime()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (t *TransactionMetadata) ObjectStorageKey() []byte {
	return t.id.Bytes()
}

// ObjectStorageValue marshals the TransactionMetadata into a sequence of bytes. The ID is not serialized here as it is
// only used as a key in the ObjectStorage.
func (t *TransactionMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(t.BranchIDs()).
		WriteBool(t.Solid()).
		WriteTime(t.SolidificationTime()).
		WriteBool(t.LazyBooked()).
		WriteUint8(uint8(t.GradeOfFinality())).
		WriteTime(t.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &TransactionMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object storage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{utxo.OutputIDLength, utxo.TransactionIDLength}...)

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	consumedInput utxo.OutputID
	transactionID utxo.TransactionID
	validMutex    sync.RWMutex
	valid         types.TriBool

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput utxo.OutputID, transactionID utxo.TransactionID, valid types.TriBool) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
		valid:         valid,
	}
}

// FromObjectStorage creates an Consumer from sequences of key and bytes.
func (c *Consumer) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	result, err := c.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse Consumer from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a Consumer from a sequence of bytes.
func (c *Consumer) FromBytes(bytes []byte) (consumer *Consumer, err error) {
	marshalUtil := marshalutil.New(bytes)
	if consumer, err = c.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse Consumer from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals a Consumer using a MarshalUtil (for easier unmarshalling).
func (c *Consumer) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (consumer *Consumer, err error) {
	if consumer = c; consumer == nil {
		consumer = new(Consumer)
	}
	if consumer.consumedInput, err = utxo.OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
		return
	}
	if consumer.transactionID, err = utxo.TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	if consumer.valid, err = types.TriBoolFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse valid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ConsumedInput returns the OutputID of the consumed Input.
func (c *Consumer) ConsumedInput() utxo.OutputID {
	return c.consumedInput
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() utxo.TransactionID {
	return c.transactionID
}

// Valid returns a flag that indicates if the spending Transaction is valid or not.
func (c *Consumer) Valid() (valid types.TriBool) {
	c.validMutex.RLock()
	defer c.validMutex.RUnlock()

	return c.valid
}

// SetValid updates the valid flag of the Consumer and returns true if the value was changed.
func (c *Consumer) SetValid(valid types.TriBool) (updated bool) {
	c.validMutex.Lock()
	defer c.validMutex.Unlock()

	if valid == c.valid {
		return
	}

	c.valid = valid
	c.SetModified()
	updated = true

	return
}

// Bytes marshals the Consumer into a sequence of bytes.
func (c *Consumer) Bytes() []byte {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the Consumer.
func (c *Consumer) String() (humanReadableConsumer string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("transactionID", c.transactionID),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Consumer) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(c.consumedInput.Bytes(), c.transactionID.Bytes())
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (c *Consumer) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.BoolSize).
		Write(c.Valid()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Consumer)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
