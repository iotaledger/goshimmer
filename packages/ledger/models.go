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
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region Transaction ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Transaction represents a wrapped utxo.Transaction that can be stored in the object storage.
type Transaction struct {
	// Transaction contains the wrapped utxo.Transaction.
	utxo.Transaction

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewTransaction returns a new Transaction from the given utxo.Transaction.
func NewTransaction(transaction utxo.Transaction) (new *Transaction) {
	return &Transaction{
		Transaction: transaction,
	}
}

// FromObjectStorage un-serializes a Transaction from an object storage (it is disabled because we use the VMs parser).
func (o *Transaction) FromObjectStorage([]byte, []byte) (transaction objectstorage.StorableObject, err error) {
	panic("this should never be called - we use a custom factory method from the VM")
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (o *Transaction) ObjectStorageKey() (key []byte) {
	return o.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (o *Transaction) ObjectStorageValue() []byte {
	return o.Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Transaction)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionMetadata //////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata represents a container for additional information about a Transaction.
type TransactionMetadata struct {
	// id contains the identifier of the Transaction.
	id utxo.TransactionID

	// branchIDs contains the conflicting BranchIDs that this Transaction depends on.
	branchIDs branchdag.BranchIDs

	// branchIDsMutex contains a mutex that is used to synchronize parallel access to the branchIDs.
	branchIDsMutex sync.RWMutex

	// booked contains a boolean flag that indicates if the Transaction was booked already.
	booked bool

	// branchIDsMutex contains a mutex that is used to synchronize parallel access to the branchIDs.
	bookedMutex sync.RWMutex

	// bookingTime contains the time the Transaction was booked.
	bookingTime time.Time

	// bookingTimeMutex contains a mutex that is used to synchronize parallel access to the bookingTime.
	bookingTimeMutex sync.RWMutex

	// outputIDs contains the identifiers of the Outputs that the Transaction created.
	outputIDs utxo.OutputIDs

	// outputIDsMutex contains a mutex that is used to synchronize parallel access to the outputIDs.
	outputIDsMutex sync.RWMutex

	// gradeOfFinality contains the confirmation status of the Transaction.
	gradeOfFinality gof.GradeOfFinality

	// gradeOfFinalityTime contains the last time the gradeOfFinality was updated.
	gradeOfFinalityTime time.Time

	// gradeOfFinalityMutex contains a mutex that is used to synchronize parallel access to the gradeOfFinality.
	gradeOfFinalityMutex sync.RWMutex

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewTransactionMetadata returns new TransactionMetadata for the given TransactionID.
func NewTransactionMetadata(txID utxo.TransactionID) (new *TransactionMetadata) {
	new = &TransactionMetadata{
		id:        txID,
		branchIDs: branchdag.NewBranchIDs(),
		outputIDs: utxo.NewOutputIDs(),
	}
	new.SetModified()
	new.Persist()

	return new
}

// FromObjectStorage un-serializes TransactionMetadata from an object storage.
func (t *TransactionMetadata) FromObjectStorage(key, bytes []byte) (transactionMetadata objectstorage.StorableObject, err error) {
	result := new(TransactionMetadata)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse TransactionMetadata from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes TransactionMetadata from a sequence of bytes.
func (t *TransactionMetadata) FromBytes(bytes []byte) (err error) {
	if err = t.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse TransactionMetadata from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes TransactionMetadata using a MarshalUtil.
func (t *TransactionMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	t.branchIDs = branchdag.NewBranchIDs()
	t.outputIDs = utxo.NewOutputIDs()

	if err = t.id.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse TransactionID: %w", err)
	}
	if err = t.branchIDs.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse BranchID: %w", err)
	}
	if t.booked, err = marshalUtil.ReadBool(); err != nil {
		return errors.Errorf("failed to parse solid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if t.bookingTime, err = marshalUtil.ReadTime(); err != nil {
		return errors.Errorf("failed to parse solidify time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	if err = t.outputIDs.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse OutputIDs: %w", err)
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		return errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	t.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if t.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		return errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return nil
}

// ID returns the identifier of the Transaction that this TransactionMetadata belongs to.
func (t *TransactionMetadata) ID() (id utxo.TransactionID) {
	return t.id
}

// BranchIDs returns the conflicting BranchIDs that the Transaction depends on.
func (t *TransactionMetadata) BranchIDs() (branchIDs branchdag.BranchIDs) {
	t.branchIDsMutex.RLock()
	defer t.branchIDsMutex.RUnlock()

	return t.branchIDs.Clone()
}

// SetBranchIDs sets the conflicting BranchIDs that this Transaction depends on.
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

// IsBooked returns a boolean flag indicating whether the Transaction has been booked.
func (t *TransactionMetadata) IsBooked() (booked bool) {
	t.bookedMutex.RLock()
	defer t.bookedMutex.RUnlock()

	return t.booked
}

// SetBooked sets a boolean flag indicating whether the Transaction has been booked.
func (t *TransactionMetadata) SetBooked(booked bool) (modified bool) {
	t.bookedMutex.Lock()
	defer t.bookedMutex.Unlock()

	if t.booked == booked {
		return
	}

	if booked {
		t.bookingTimeMutex.Lock()
		t.bookingTime = time.Now()
		t.bookingTimeMutex.Unlock()
	}

	t.booked = booked
	t.SetModified()

	return true
}

// BookingTime returns the time when the Transaction was booked.
func (t *TransactionMetadata) BookingTime() (bookingTime time.Time) {
	t.bookingTimeMutex.RLock()
	defer t.bookingTimeMutex.RUnlock()

	return t.bookingTime
}

// OutputIDs returns the identifiers of the Outputs that the Transaction created.
func (t *TransactionMetadata) OutputIDs() (outputIDs utxo.OutputIDs) {
	t.outputIDsMutex.RLock()
	defer t.outputIDsMutex.RUnlock()

	return t.outputIDs.Clone()
}

// SetOutputIDs sets the identifiers of the Outputs that the Transaction created.
func (t *TransactionMetadata) SetOutputIDs(outputIDs utxo.OutputIDs) (modified bool) {
	t.outputIDsMutex.RLock()
	defer t.outputIDsMutex.RUnlock()

	if t.outputIDs.Equal(outputIDs) {
		return false
	}

	t.outputIDs = outputIDs
	t.SetModified()

	return true
}

// GradeOfFinality returns the confirmation status of the Transaction.
func (t *TransactionMetadata) GradeOfFinality() (gradeOfFinality gof.GradeOfFinality) {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()

	return t.gradeOfFinality
}

// SetGradeOfFinality sets the confirmation status of the Transaction.
func (t *TransactionMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	t.gradeOfFinalityMutex.Lock()
	defer t.gradeOfFinalityMutex.Unlock()

	if t.gradeOfFinality == gradeOfFinality {
		return
	}

	t.gradeOfFinality = gradeOfFinality
	t.gradeOfFinalityTime = clock.SyncedTime()
	t.SetModified()

	return true
}

// GradeOfFinalityTime returns the last time the GradeOfFinality was updated.
func (t *TransactionMetadata) GradeOfFinalityTime() (gradeOfFinalityTime time.Time) {
	t.gradeOfFinalityMutex.RLock()
	defer t.gradeOfFinalityMutex.RUnlock()

	return t.gradeOfFinalityTime
}

// IsConflicting returns true if the Transaction is conflicting with another Transaction (is a Branch).
func (t *TransactionMetadata) IsConflicting() (isConflicting bool) {
	return t.BranchIDs().Is(branchdag.NewBranchID(t.ID()))
}

// Bytes returns a serialized version of the TransactionMetadata.
func (t *TransactionMetadata) Bytes() (serialized []byte) {
	return byteutils.ConcatBytes(t.ObjectStorageKey(), t.ObjectStorageValue())
}

// String returns a human-readable version of the TransactionMetadata.
func (t *TransactionMetadata) String() (humanReadable string) {
	return stringify.Struct("TransactionMetadata",
		stringify.StructField("id", t.ID()),
		stringify.StructField("branchID", t.BranchIDs()),
		stringify.StructField("booked", t.IsBooked()),
		stringify.StructField("bookingTime", t.BookingTime()),
		stringify.StructField("outputIDs", t.OutputIDs()),
		stringify.StructField("gradeOfFinality", t.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", t.GradeOfFinalityTime()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (t *TransactionMetadata) ObjectStorageKey() (key []byte) {
	return t.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (t *TransactionMetadata) ObjectStorageValue() (value []byte) {
	return marshalutil.New().
		Write(t.BranchIDs()).
		WriteBool(t.IsBooked()).
		WriteTime(t.BookingTime()).
		Write(t.OutputIDs()).
		WriteUint8(uint8(t.GradeOfFinality())).
		WriteTime(t.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(TransactionMetadata)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Output ///////////////////////////////////////////////////////////////////////////////////////////////////////

// Output is a wrapped utxo.Output that can be stored in the object storage.
type Output struct {
	// Output contains the wrapped utxo.Output.
	utxo.Output

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewOutput returns a new Output from the given utxo.Output.
func NewOutput(output utxo.Output) (new *Output) {
	return &Output{
		Output: output,
	}
}

// FromObjectStorage un-serializes an Output from an object storage (it is disabled because we use the VMs parser).
func (o *Output) FromObjectStorage([]byte, []byte) (objectstorage.StorableObject, error) {
	panic("this should never be called - we use a custom factory method from the VM")
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (o *Output) ObjectStorageKey() (key []byte) {
	return o.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (o *Output) ObjectStorageValue() (value []byte) {
	return o.Bytes()
}

// utxoOutput returns the wrapped utxo.Output.
func (o *Output) utxoOutput() (unwrapped utxo.Output) {
	return o.Output
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Output)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Outputs //////////////////////////////////////////////////////////////////////////////////////////////////////

// Outputs represents a collection of Output objects indexed by their OutputID.
type Outputs struct {
	// OrderedMap is the underlying data structure that holds the Outputs.
	*orderedmap.OrderedMap[utxo.OutputID, *Output]
}

// NewOutputs returns a new Output collection with the given elements.
func NewOutputs(outputs ...*Output) (new Outputs) {
	new = Outputs{orderedmap.New[utxo.OutputID, *Output]()}
	for _, output := range outputs {
		new.Set(output.ID(), output)
	}

	return new
}

// IDs returns the identifiers of the stored Outputs.
func (o Outputs) IDs() (ids utxo.OutputIDs) {
	outputIDs := make([]utxo.OutputID, 0)
	o.OrderedMap.ForEach(func(id utxo.OutputID, _ *Output) bool {
		outputIDs = append(outputIDs, id)
		return true
	})

	return utxo.NewOutputIDs(outputIDs...)
}

// ForEach executes the callback for each element in the collection (it aborts if the callback returns an error).
func (o Outputs) ForEach(callback func(output *Output) error) (err error) {
	o.OrderedMap.ForEach(func(_ utxo.OutputID, output *Output) bool {
		if err = callback(output); err != nil {
			return false
		}

		return true
	})

	return err
}

// utxoOutputs returns a slice of unwrapped Outputs.
func (o Outputs) utxoOutputs() (slice []utxo.Output) {
	slice = make([]utxo.Output, 0)
	_ = o.ForEach(func(output *Output) error {
		slice = append(slice, output.Output)
		return nil
	})

	return slice
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata represents a container for additional information about an Output.
type OutputMetadata struct {
	// id contains the identifier of the Output.
	id utxo.OutputID

	// branchIDs contains the conflicting BranchIDs that this Output depends on.
	branchIDs branchdag.BranchIDs

	// branchIDsMutex contains a mutex that is used to synchronize parallel access to the branchIDs.
	branchIDsMutex sync.RWMutex

	// firstConsumer contains the first Transaction that ever spent the Output.
	firstConsumer utxo.TransactionID

	// firstConsumerForked contains a boolean flag that indicates if the firstConsumer was forked.
	firstConsumerForked bool

	// firstConsumerMutex contains a mutex that is used to synchronize parallel access to the firstConsumer.
	firstConsumerMutex sync.RWMutex

	// gradeOfFinality contains the confirmation status of the Output.
	gradeOfFinality gof.GradeOfFinality

	// gradeOfFinalityTime contains the last time the gradeOfFinality was updated.
	gradeOfFinalityTime time.Time

	// gradeOfFinalityMutex contains a mutex that is used to synchronize parallel access to the gradeOfFinality.
	gradeOfFinalityMutex sync.RWMutex

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewOutputMetadata returns new OutputMetadata for the given OutputID.
func NewOutputMetadata(outputID utxo.OutputID) (new *OutputMetadata) {
	return &OutputMetadata{
		id:        outputID,
		branchIDs: branchdag.NewBranchIDs(),
	}
}

// FromObjectStorage un-serializes OutputMetadata from an object storage.
func (o *OutputMetadata) FromObjectStorage(key, bytes []byte) (outputMetadata objectstorage.StorableObject, err error) {
	result := new(OutputMetadata)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse OutputMetadata from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes TransactionMetadata from a sequence of bytes.
func (o *OutputMetadata) FromBytes(bytes []byte) (err error) {
	if err = o.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse OutputMetadata from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes TransactionMetadata using a MarshalUtil.
func (o *OutputMetadata) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	o.branchIDs = branchdag.NewBranchIDs()

	if err = o.id.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse OutputID: %w", err)
	}
	if err = o.branchIDs.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse BranchIDs: %w", err)
	}
	if err = o.firstConsumer.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse first consumer (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	gradeOfFinality, err := marshalUtil.ReadUint8()
	if err != nil {
		return errors.Errorf("failed to parse grade of finality (%v): %w", err, cerrors.ErrParseBytesFailed)
	}
	o.gradeOfFinality = gof.GradeOfFinality(gradeOfFinality)
	if o.gradeOfFinalityTime, err = marshalUtil.ReadTime(); err != nil {
		return errors.Errorf("failed to parse gradeOfFinality time (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return nil
}

// ID returns the identifier of the Output that this OutputMetadata belongs to.
func (o *OutputMetadata) ID() (id utxo.OutputID) {
	return o.id
}

// BranchIDs returns the conflicting BranchIDs that the Output depends on.
func (o *OutputMetadata) BranchIDs() (branchIDs branchdag.BranchIDs) {
	o.branchIDsMutex.RLock()
	defer o.branchIDsMutex.RUnlock()

	return o.branchIDs.Clone()
}

// SetBranchIDs sets the conflicting BranchIDs that this Transaction depends on.
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

// FirstConsumer returns the first Transaction that ever spent the Output.
func (o *OutputMetadata) FirstConsumer() (firstConsumer utxo.TransactionID) {
	o.firstConsumerMutex.RLock()
	defer o.firstConsumerMutex.RUnlock()

	return o.firstConsumer
}

// RegisterBookedConsumer registers a booked consumer and checks if it is conflicting with another consumer that wasn't
// forked, yet.
func (o *OutputMetadata) RegisterBookedConsumer(consumer utxo.TransactionID) (isConflicting bool, consumerToFork utxo.TransactionID) {
	o.firstConsumerMutex.Lock()
	defer o.firstConsumerMutex.Unlock()

	if o.firstConsumer == utxo.EmptyTransactionID {
		o.firstConsumer = consumer
		o.SetModified()

		return false, utxo.EmptyTransactionID
	}

	if o.firstConsumerForked {
		return true, utxo.EmptyTransactionID
	}

	return true, o.firstConsumer
}

// GradeOfFinality returns the confirmation status of the Output.
func (o *OutputMetadata) GradeOfFinality() (gradeOfFinality gof.GradeOfFinality) {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()

	return o.gradeOfFinality
}

// SetGradeOfFinality sets the confirmation status of the Output.
func (o *OutputMetadata) SetGradeOfFinality(gradeOfFinality gof.GradeOfFinality) (modified bool) {
	o.gradeOfFinalityMutex.Lock()
	defer o.gradeOfFinalityMutex.Unlock()

	if o.gradeOfFinality == gradeOfFinality {
		return false
	}

	o.gradeOfFinality = gradeOfFinality
	o.gradeOfFinalityTime = clock.SyncedTime()
	o.SetModified()

	return true
}

// GradeOfFinalityTime returns the last time the GradeOfFinality was updated.
func (o *OutputMetadata) GradeOfFinalityTime() (gradeOfFinality time.Time) {
	o.gradeOfFinalityMutex.RLock()
	defer o.gradeOfFinalityMutex.RUnlock()

	return o.gradeOfFinalityTime
}

// IsSpent returns true if the Output has been spent.
func (o *OutputMetadata) IsSpent() (isSpent bool) {
	o.firstConsumerMutex.RLock()
	defer o.firstConsumerMutex.RUnlock()

	return o.firstConsumer != utxo.EmptyTransactionID
}

// Bytes returns a serialized version of the OutputMetadata.
func (o *OutputMetadata) Bytes() (serialized []byte) {
	return byteutils.ConcatBytes(o.ObjectStorageKey(), o.ObjectStorageValue())
}

// String returns a human-readable version of the OutputMetadata.
func (o *OutputMetadata) String() (humanReadable string) {
	return stringify.Struct("OutputMetadata",
		stringify.StructField("id", o.ID()),
		stringify.StructField("branchIDs", o.BranchIDs()),
		stringify.StructField("firstConsumer", o.FirstConsumer()),
		stringify.StructField("gradeOfFinality", o.GradeOfFinality()),
		stringify.StructField("gradeOfFinalityTime", o.GradeOfFinalityTime()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (o *OutputMetadata) ObjectStorageKey() (key []byte) {
	return o.ID().Bytes()
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (o *OutputMetadata) ObjectStorageValue() (value []byte) {
	return marshalutil.New().
		Write(o.BranchIDs()).
		Write(o.FirstConsumer()).
		WriteUint8(uint8(o.GradeOfFinality())).
		WriteTime(o.GradeOfFinalityTime()).
		Bytes()
}

// code contract (make sure the type implements all required methods)
var _ objectstorage.StorableObject = new(OutputMetadata)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// OutputsMetadata represents a collection of OutputMetadata objects indexed by their OutputID.
type OutputsMetadata struct {
	// OrderedMap is the underlying data structure that holds the OutputMetadata objects.
	*orderedmap.OrderedMap[utxo.OutputID, *OutputMetadata]
}

// NewOutputsMetadata returns a new OutputMetadata collection with the given elements.
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

// IDs returns the identifiers of the stored OutputMetadata objects.
func (o OutputsMetadata) IDs() (ids utxo.OutputIDs) {
	ids = utxo.NewOutputIDs()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		ids.Add(outputMetadata.ID())
		return nil
	})

	return ids
}

// BranchIDs returns a union of all BranchIDs of the contained OutputMetadata objects.
func (o OutputsMetadata) BranchIDs() (branchIDs branchdag.BranchIDs) {
	branchIDs = branchdag.NewBranchIDs()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		branchIDs.AddAll(outputMetadata.BranchIDs())
		return nil
	})

	return branchIDs
}

// ForEach executes the callback for each element in the collection (it aborts if the callback returns an error).
func (o OutputsMetadata) ForEach(callback func(outputMetadata *OutputMetadata) error) (err error) {
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

// Consumer represents the reference between an Output and its spending Transaction.
type Consumer struct {
	// consumedInput contains the identifier of the Output that was spent.
	consumedInput utxo.OutputID

	// transactionID contains the identifier of the spending Transaction.
	transactionID utxo.TransactionID

	// booked contains a boolean flag that indicates whether the Consumer was completely booked.
	booked bool

	// bookedMutex contains a mutex that is used to synchronize parallel access to the booked flag.
	bookedMutex sync.RWMutex

	// StorableObjectFlags embeds the properties and methods required to manage the object storage related flags.
	objectstorage.StorableObjectFlags
}

// NewConsumer return a new Consumer reference from the named Output to the named Transaction.
func NewConsumer(consumedInput utxo.OutputID, transactionID utxo.TransactionID) (new *Consumer) {
	new = &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
	}

	new.Persist()
	new.SetModified()

	return new
}

// FromObjectStorage un-serializes a Consumer from an object storage.
func (c *Consumer) FromObjectStorage(key, bytes []byte) (consumer objectstorage.StorableObject, err error) {
	result := new(Consumer)
	if err = result.FromBytes(byteutils.ConcatBytes(key, bytes)); err != nil {
		return nil, errors.Errorf("failed to parse Consumer from bytes: %w", err)
	}

	return result, nil
}

// FromBytes un-serializes a Consumer from a sequence of bytes.
func (c *Consumer) FromBytes(bytes []byte) (err error) {
	if err = c.FromMarshalUtil(marshalutil.New(bytes)); err != nil {
		return errors.Errorf("failed to parse Consumer from MarshalUtil: %w", err)
	}

	return nil
}

// FromMarshalUtil un-serializes a Consumer using a MarshalUtil.
func (c *Consumer) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (err error) {
	if err = c.consumedInput.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
	}
	if err = c.transactionID.FromMarshalUtil(marshalUtil); err != nil {
		return errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
	}
	if c.booked, err = marshalUtil.ReadBool(); err != nil {
		return errors.Errorf("failed to parse processed flag (%v): %w", err, cerrors.ErrParseBytesFailed)
	}

	return nil
}

// ConsumedInput returns the identifier of the Output that was spent.
func (c *Consumer) ConsumedInput() (outputID utxo.OutputID) {
	return c.consumedInput
}

// TransactionID returns the identifier of the spending Transaction.
func (c *Consumer) TransactionID() (spendingTransaction utxo.TransactionID) {
	return c.transactionID
}

// IsBooked returns a boolean flag that indicates whether the Consumer was completely booked.
func (c *Consumer) IsBooked() (processed bool) {
	c.bookedMutex.RLock()
	defer c.bookedMutex.RUnlock()

	return c.booked
}

// SetBooked sets a boolean flag that indicates whether the Consumer was completely booked.
func (c *Consumer) SetBooked() (updated bool) {
	c.bookedMutex.Lock()
	defer c.bookedMutex.Unlock()

	if c.booked {
		return
	}

	c.booked = true
	c.SetModified()
	updated = true

	return
}

// Bytes returns a serialized version of the Consumer.
func (c *Consumer) Bytes() (serialized []byte) {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human-readable version of the Consumer.
func (c *Consumer) String() (humanReadable string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("transactionID", c.transactionID),
		stringify.StructField("processed", c.IsBooked()),
	)
}

// ObjectStorageKey serializes the part of the object that is stored in the key part of the object storage.
func (c *Consumer) ObjectStorageKey() (key []byte) {
	return byteutils.ConcatBytes(c.consumedInput.Bytes(), c.transactionID.Bytes())
}

// ObjectStorageValue serializes the part of the object that is stored in the value part of the object storage.
func (c *Consumer) ObjectStorageValue() (value []byte) {
	return marshalutil.New(marshalutil.BoolSize).
		WriteBool(c.IsBooked()).
		Bytes()
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Consumer)

// consumerPartitionKeys defines the partition of the storage key of the Consumer model.
var consumerPartitionKeys = objectstorage.PartitionKey([]int{utxo.OutputIDLength, utxo.TransactionIDLength}...)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
