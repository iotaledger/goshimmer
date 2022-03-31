package ledger

import (
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/refactored/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region Transaction ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Transaction struct {
	utxo.Transaction
	objectstorage.StorableObjectFlags
}

func NewTransaction(transaction utxo.Transaction) (new *Transaction) {
	return &Transaction{
		Transaction: transaction,
	}
}

func (o *Transaction) FromObjectStorage([]byte, []byte) (objectstorage.StorableObject, error) {
	panic("this should never be called - we use a custom factory method from the VM")
}

func (o *Transaction) ObjectStorageKey() []byte {
	return o.ID().Bytes()
}

func (o *Transaction) ObjectStorageValue() []byte {
	return o.Bytes()
}

var _ objectstorage.StorableObject = new(Transaction)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata //////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata contains additional information about a Transaction that is derived from the local perception of
// a node.
type TransactionMetadata struct {
	id                      *utxo.TransactionID
	branchIDs               branchdag.BranchIDs
	branchIDsMutex          sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	lazyBooked              bool
	lazyBookedMutex         sync.RWMutex
	outputIDs               utxo.OutputIDs
	outputIDsMutex          sync.RWMutex
	gradeOfFinality         gof.GradeOfFinality
	gradeOfFinalityTime     time.Time
	gradeOfFinalityMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata creates a new empty TransactionMetadata object.
func NewTransactionMetadata(transactionID *utxo.TransactionID) (newTransactionMetadata *TransactionMetadata) {
	newTransactionMetadata = &TransactionMetadata{
		id:        transactionID,
		branchIDs: branchdag.NewBranchIDs(),
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

	if err = transactionMetadata.id.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse TransactionID: %w", err)
	}
	if err = transactionMetadata.branchIDs.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse BranchID: %w", err)
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		return nil, errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return nil, errors.Errorf("failed to parse solidify time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if transactionMetadata.lazyBooked, err = marshalUtil.ReadBool(); err != nil {
		return nil, errors.Errorf("failed to parse lazy booked flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if err = transactionMetadata.outputIDs.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse OutputIDs: %w", err)
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		return nil, errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	transactionMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if transactionMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		return nil, errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return
}

// ID returns the TransactionID of the Transaction that the TransactionMetadata belongs to.
func (t *TransactionMetadata) ID() *utxo.TransactionID {
	return t.id
}

// BranchIDs returns the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) BranchIDs() branchdag.BranchIDs {
	t.branchIDsMutex.RLock()
	defer t.branchIDsMutex.RUnlock()

	return t.branchIDs.Clone()
}

// SetBranchIDs sets the identifiers of the Branches that the Transaction was booked in.
func (t *TransactionMetadata) SetBranchIDs(branchIDs branchdag.BranchIDs) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Equal(branchIDs) {
		return false
	}

	t.branchIDs = branchIDs.Clone()
	t.SetModified()
	return true
}

// AddBranchID adds an identifier of the Branch that the Transaction was booked in.
func (t *TransactionMetadata) AddBranchID(branchID *branchdag.BranchID) (modified bool) {
	t.branchIDsMutex.Lock()
	defer t.branchIDsMutex.Unlock()

	if t.branchIDs.Has(branchID) {
		return false
	}

	t.branchIDs.Delete(&branchdag.MasterBranchID)

	t.branchIDs.Add(branchID)
	t.SetModified()
	return true
}

// Booked returns true if the Transaction has been marked as solid.
func (t *TransactionMetadata) Booked() bool {
	t.solidMutex.RLock()
	defer t.solidMutex.RUnlock()

	return t.solid
}

// SetBooked updates the solid flag of the Transaction. It returns true if the solid flag was modified and updates
// the solidification time if the Transaction was marked as solid.
func (t *TransactionMetadata) SetBooked(solid bool) (modified bool) {
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

// OutputIDs returns the OutputIDs that this Transaction created.
func (t *TransactionMetadata) OutputIDs() utxo.OutputIDs {
	t.outputIDsMutex.RLock()
	defer t.outputIDsMutex.RUnlock()

	return t.outputIDs
}

// SetOutputIDs sets the OutputIDs that this Transaction created.
func (t *TransactionMetadata) SetOutputIDs(outputIDs utxo.OutputIDs) {
	t.outputIDsMutex.RLock()
	defer t.outputIDsMutex.RUnlock()

	t.outputIDs = outputIDs
	t.SetModified()

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
	return t.BranchIDs().Is(t.ID())
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
		stringify.StructField("processed", t.Booked()),
		stringify.StructField("solidificationTime", t.SolidificationTime()),
		stringify.StructField("lazyBooked", t.LazyBooked()),
		stringify.StructField("outputIDs", t.OutputIDs()),
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
		WriteBool(t.Booked()).
		WriteTime(t.SolidificationTime()).
		WriteBool(t.LazyBooked()).
		Write(t.OutputIDs()).
		WriteUint8(uint8(t.GradeOfFinality())).
		WriteTime(t.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = &TransactionMetadata{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

type Output struct {
	utxo.Output
	objectstorage.StorableObjectFlags
}

func NewOutput(output utxo.Output) (new *Output) {
	return &Output{
		Output: output,
	}
}

func (o *Output) FromObjectStorage([]byte, []byte) (objectstorage.StorableObject, error) {
	panic("this should never be called - we use a custom factory method from the VM")
}

func (o *Output) ObjectStorageKey() []byte {
	return o.ID().Bytes()
}

func (o *Output) ObjectStorageValue() []byte {
	return o.Bytes()
}

func (o *Output) utxoOutput() utxo.Output {
	return o.Output
}

func (o *Output) ID() (id utxo.OutputID) {
	return o.Output.ID()
}

var _ objectstorage.StorableObject = new(Output)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

type Outputs struct {
	*orderedmap.OrderedMap[utxo.OutputID, *Output]
}

func NewOutputs(outputs ...*Output) (new Outputs) {
	new = Outputs{orderedmap.New[utxo.OutputID, *Output]()}
	for _, output := range outputs {
		new.Set(output.ID(), output)
	}

	return new
}

func (o Outputs) IDs() (ids utxo.OutputIDs) {
	outputIDs := make([]utxo.OutputID, 0)
	o.OrderedMap.ForEach(func(id utxo.OutputID, _ *Output) bool {
		outputIDs = append(outputIDs, id)
		return true
	})

	return utxo.NewOutputIDs(outputIDs...)
}

func (o Outputs) ForEach(callback func(output *Output) (err error)) (err error) {
	o.OrderedMap.ForEach(func(_ utxo.OutputID, output *Output) bool {
		if err = callback(output); err != nil {
			return false
		}

		return true
	})

	return err
}

func (o Outputs) UTXOOutputs() (slice []utxo.Output) {
	slice = make([]utxo.Output, 0)
	_ = o.ForEach(func(output *Output) error {
		slice = append(slice, output.Output)
		return nil
	})

	return slice
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata contains additional Output information that are derived from the local perception of the node.
type OutputMetadata struct {
	id                      utxo.OutputID
	branchIDs               branchdag.BranchIDs
	branchIDsMutex          sync.RWMutex
	solid                   bool
	solidMutex              sync.RWMutex
	solidificationTime      time.Time
	solidificationTimeMutex sync.RWMutex
	firstConsumer           *utxo.TransactionID
	firstConsumerForked     bool
	firstConsumerMutex      sync.RWMutex
	gradeOfFinality         gof.GradeOfFinality
	gradeOfFinalityTime     time.Time
	gradeOfFinalityMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewOutputMetadata creates a new empty OutputMetadata object.
func NewOutputMetadata(outputID utxo.OutputID) *OutputMetadata {
	return &OutputMetadata{
		id:        outputID,
		branchIDs: branchdag.NewBranchIDs(),
	}
}

// FromObjectStorage creates an OutputMetadata from sequences of key and bytes.
func (o *OutputMetadata) FromObjectStorage(key, bytes []byte) (objectstorage.StorableObject, error) {
	outputMetadata, err := o.FromBytes(byteutils.ConcatBytes(key, bytes))
	if err != nil {
		err = errors.Errorf("failed to parse OutputMetadata from bytes: %w", err)
	}
	return outputMetadata, err
}

// FromBytes unmarshals an OutputMetadata object from a sequence of bytes.
func (o *OutputMetadata) FromBytes(bytes []byte) (outputMetadata *OutputMetadata, err error) {
	marshalUtil := marshalutil.New(bytes)
	if outputMetadata, err = o.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputMetadata from MarshalUtil: %w", err)
		return
	}

	return
}

// FromMarshalUtil unmarshals an OutputMetadata object using a MarshalUtil (for easier unmarshalling).
func (o *OutputMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (outputMetadata *OutputMetadata, err error) {
	if outputMetadata = o; outputMetadata == nil {
		outputMetadata = &OutputMetadata{}
	}

	if err = outputMetadata.id.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse OutputID: %w", err)
	}
	if err = outputMetadata.branchIDs.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse BranchIDs: %w", err)
	}
	if outputMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		return nil, errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if outputMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return nil, errors.Errorf("failed to parse solidification time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if err = outputMetadata.firstConsumer.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse first consumer (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		return nil, errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	outputMetadata.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if outputMetadata.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		err = errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}
	return
}

// ID returns the OutputID of the Output that the OutputMetadata belongs to.
func (o *OutputMetadata) ID() utxo.OutputID {
	return o.id
}

// BranchIDs returns the identifiers of the Branches that the Output was booked in.
func (o *OutputMetadata) BranchIDs() branchdag.BranchIDs {
	o.branchIDsMutex.RLock()
	defer o.branchIDsMutex.RUnlock()

	return o.branchIDs.Clone()
}

// SetBranchIDs sets the identifiers of the Branches that the Output was booked in.
func (o *OutputMetadata) SetBranchIDs(branchIDs branchdag.BranchIDs) (modified bool) {
	o.branchIDsMutex.Lock()
	defer o.branchIDsMutex.Unlock()

	if o.branchIDs.Equal(branchIDs) {
		return false
	}

	o.branchIDs = branchIDs.Clone()
	o.SetModified()
	return true
}

// AddBranchID adds an identifier of the Branch that the Output was booked in.
func (o *OutputMetadata) AddBranchID(branchID *branchdag.BranchID) (modified bool) {
	o.branchIDsMutex.Lock()
	defer o.branchIDsMutex.Unlock()

	if o.branchIDs.Has(branchID) {
		return false
	}

	o.branchIDs.Delete(&branchdag.MasterBranchID)

	o.branchIDs.Add(branchID)
	o.SetModified()
	modified = true

	return
}

// Solid returns true if the Output has been marked as solid.
func (o *OutputMetadata) Solid() bool {
	o.solidMutex.RLock()
	defer o.solidMutex.RUnlock()

	return o.solid
}

// SetSolid updates the solid flag of the Output. It returns true if the solid flag was modified and updates the
// solidification time if the Output was marked as solid.
func (o *OutputMetadata) SetSolid(solid bool) (modified bool) {
	o.solidMutex.Lock()
	defer o.solidMutex.Unlock()

	if o.solid == solid {
		return
	}

	if solid {
		o.solidificationTimeMutex.Lock()
		o.solidificationTime = time.Now()
		o.solidificationTimeMutex.Unlock()
	}

	o.solid = solid
	o.SetModified()
	modified = true

	return
}

// SolidificationTime returns the time when the Output was marked as solid.
func (o *OutputMetadata) SolidificationTime() time.Time {
	o.solidificationTimeMutex.RLock()
	defer o.solidificationTimeMutex.RUnlock()

	return o.solidificationTime
}

// Spent returns true if the Output has been spent already.
func (o *OutputMetadata) Spent() bool {
	o.firstConsumerMutex.RLock()
	defer o.firstConsumerMutex.RUnlock()

	return o.firstConsumer != nil && *o.firstConsumer != utxo.EmptyTransactionID
}

// FirstConsumer returns the TransactionID that first spent the Output (or the EmptyTransactionID if it is unspent).
func (o *OutputMetadata) FirstConsumer() *utxo.TransactionID {
	o.firstConsumerMutex.RLock()
	defer o.firstConsumerMutex.RUnlock()

	return o.firstConsumer
}

// RegisterProcessedConsumer increases the consumer count of an Output and stores the first Consumer that was ever registered. It
// returns the previous consumer count.
func (o *OutputMetadata) RegisterProcessedConsumer(consumer *utxo.TransactionID) (isConflicting bool, consumerToFork *utxo.TransactionID) {
	o.firstConsumerMutex.Lock()
	defer o.firstConsumerMutex.Unlock()

	if o.firstConsumer == nil || *o.firstConsumer == utxo.EmptyTransactionID {
		o.firstConsumer = consumer
		o.SetModified()

		return false, &utxo.EmptyTransactionID
	}

	if o.firstConsumerForked {
		return true, &utxo.EmptyTransactionID
	}

	return true, o.firstConsumer
}

// GradeOfFinality returns the grade of finality.
func (o *OutputMetadata) GradeOfFinality() gof.GradeOfFinality {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()
	return o.gradeOfFinality
}

// SetGradeOfFinality updates the grade of finality. It returns true if it was modified.
func (o *OutputMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	o.gradeOfFinalityMutex.Lock()
	defer o.gradeOfFinalityMutex.Unlock()

	if o.gradeOfFinality == gradeOfFinality {
		return
	}

	o.gradeOfFinality = gradeOfFinality
	o.gradeOfFinalityTime = clock.SyncedTime()
	o.SetModified()
	modified = true
	return
}

// GradeOfFinalityTime returns the time the Output's gradeOfFinality was set.
func (o *OutputMetadata) GradeOfFinalityTime() time.Time {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()
	return o.gradeOfFinalityTime
}

// Bytes marshals the OutputMetadata into a sequence of bytes.
func (o *OutputMetadata) Bytes() []byte {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human-readable version of the OutputMetadata.
func (o *OutputMetadata) String() string {
	return stringify.Struct("OutputMetadata",
		stringify.StructField("id", o.ID()),
		stringify.StructField("branchIDs", o.BranchIDs()),
		stringify.StructField("solid", o.Solid()),
		stringify.StructField("solidificationTime", o.SolidificationTime()),
		stringify.StructField("firstConsumer", o.FirstConsumer()),
		stringify.StructField("gradeOfFinality", o.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", o.GradeOfFinalityTime()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (o *OutputMetadata) ObjectStorageKey() []byte {
	return o.id.Bytes()
}

// ObjectStorageValue marshals the OutputMetadata into a sequence of bytes. The ID is not serialized here as it is only
// used as a key in the ObjectStorage.
func (o *OutputMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		Write(o.BranchIDs()).
		WriteBool(o.Solid()).
		WriteTime(o.SolidificationTime()).
		Write(o.FirstConsumer()).
		WriteUint8(uint8(o.GradeOfFinality())).
		WriteTime(o.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(OutputMetadata)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadata //////////////////////////////////////////////////////////////////////////////////////////////

type OutputsMetadata struct {
	*orderedmap.OrderedMap[utxo.OutputID, *OutputMetadata]
}

func NewOutputsMetadata(outputsMetadata ...*OutputMetadata) (new OutputsMetadata) {
	new = OutputsMetadata{orderedmap.New[utxo.OutputID, *OutputMetadata]()}
	for _, outputMetadata := range outputsMetadata {
		new.Set(outputMetadata.ID(), outputMetadata)
	}

	return new
}

func (o OutputsMetadata) Filter(predicate func(outputMetadata *OutputMetadata) bool) (filtered OutputsMetadata) {
	filtered = NewOutputsMetadata()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		if predicate(outputMetadata) {
			filtered.Set(outputMetadata.ID(), outputMetadata)
		}

		return nil
	})

	return filtered
}

func (o OutputsMetadata) IDs() (ids utxo.OutputIDs) {
	ids = utxo.NewOutputIDs()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		ids.Add(outputMetadata.ID())
		return nil
	})

	return ids
}

func (o OutputsMetadata) BranchIDs() (branchIDs branchdag.BranchIDs) {
	branchIDs = branchdag.NewBranchIDs()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		branchIDs.AddAll(outputMetadata.BranchIDs())
		return nil
	})

	return branchIDs
}

func (o OutputsMetadata) ForEach(callback func(outputMetadata *OutputMetadata) (err error)) (err error) {
	o.OrderedMap.ForEach(func(_ utxo.OutputID, outputMetadata *OutputMetadata) bool {
		if err = callback(outputMetadata); err != nil {
			return false
		}

		return true
	})

	return err
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object storage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{utxo.OutputIDLength, utxo.TransactionIDLength}...)

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	consumedInput  utxo.OutputID
	transactionID  *utxo.TransactionID
	processedMutex sync.RWMutex
	processed      bool

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput utxo.OutputID, transactionID *utxo.TransactionID) (new *Consumer) {
	new = &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
	}

	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage creates a Consumer from sequences of key and bytes.
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

	if err = consumer.consumedInput.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
	}
	if err = consumer.transactionID.FromMarshalUtil(marshalUtil); err != nil {
		return nil, errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
	}
	if consumer.processed, err = marshalUtil.ReadBool(); err != nil {
		return nil, errors.Errorf("failed to parse processed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return
}

// ConsumedInput returns the OutputID of the consumed Input.
func (c *Consumer) ConsumedInput() utxo.OutputID {
	return c.consumedInput
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() *utxo.TransactionID {
	return c.transactionID
}

// Processed returns a flag that indicates if the spending Transaction is valid or not.
func (c *Consumer) Processed() (processed bool) {
	c.processedMutex.RLock()
	defer c.processedMutex.RUnlock()

	return c.processed
}

// SetBooked updates the valid flag of the Consumer and returns true if the value was changed.
func (c *Consumer) SetBooked() (updated bool) {
	c.processedMutex.Lock()
	defer c.processedMutex.Unlock()

	if c.processed {
		return
	}

	c.processed = true
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
		stringify.StructField("processed", c.Processed()),
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
		WriteBool(c.Processed()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Consumer)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
