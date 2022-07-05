package ledger

import (
	"time"

	"github.com/iotaledger/hive.go/generics/model"
	"github.com/iotaledger/hive.go/generics/orderedmap"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/clock"
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// region TransactionMetadata //////////////////////////////////////////////////////////////////////////////////////////

// TransactionMetadata represents a container for additional information about a Transaction.
type TransactionMetadata struct {
	model.Storable[utxo.TransactionID, TransactionMetadata, *TransactionMetadata, transactionMetadata] `serix:"0"`
}

type transactionMetadata struct {
	// BranchIDs contains the conflicting BranchIDs that this Transaction depends on.
	BranchIDs utxo.TransactionIDs `serix:"0"`

	// Booked contains a boolean flag that indicates if the Transaction was Booked already.
	Booked bool `serix:"1"`

	// BookingTime contains the time the Transaction was Booked.
	BookingTime time.Time `serix:"2"`

	// InclusionTime contains the timestamp of the earliest included attachment of this transaction in the tangle.
	InclusionTime time.Time `serix:"3"`

	// OutputIDs contains the identifiers of the Outputs that the Transaction created.
	OutputIDs utxo.OutputIDs `serix:"4"`

	// ConfirmationState contains the confirmation status of the Transaction.
	ConfirmationState confirmation.State `serix:"5"`

	// ConfirmationStateTime contains the last time the ConfirmationState was updated.
	ConfirmationStateTime time.Time `serix:"6"`
}

// NewTransactionMetadata returns new TransactionMetadata for the given TransactionID.
func NewTransactionMetadata(txID utxo.TransactionID) (new *TransactionMetadata) {
	new = model.NewStorable[utxo.TransactionID, TransactionMetadata](&transactionMetadata{
		BranchIDs: utxo.NewTransactionIDs(),
		OutputIDs: utxo.NewOutputIDs(),
	})
	new.SetID(txID)

	return new
}

// BranchIDs returns the conflicting BranchIDs that the Transaction depends on.
func (t *TransactionMetadata) BranchIDs() (branchIDs *set.AdvancedSet[utxo.TransactionID]) {
	t.RLock()
	defer t.RUnlock()

	return t.M.BranchIDs.Clone()
}

// SetBranchIDs sets the conflicting BranchIDs that this Transaction depends on.
func (t *TransactionMetadata) SetBranchIDs(branchIDs *set.AdvancedSet[utxo.TransactionID]) (modified bool) {
	t.Lock()
	defer t.Unlock()

	if t.M.BranchIDs.Equal(branchIDs) {
		return false
	}

	t.M.BranchIDs = branchIDs.Clone()
	t.SetModified()

	return true
}

// IsBooked returns a boolean flag indicating whether the Transaction has been booked.
func (t *TransactionMetadata) IsBooked() (booked bool) {
	t.RLock()
	defer t.RUnlock()

	return t.M.Booked
}

// SetBooked sets a boolean flag indicating whether the Transaction has been booked.
func (t *TransactionMetadata) SetBooked(booked bool) (modified bool) {
	t.Lock()
	defer t.Unlock()

	if t.M.Booked == booked {
		return
	}

	if booked {
		t.M.BookingTime = time.Now()
	}

	t.M.Booked = booked
	t.SetModified()

	return true
}

// BookingTime returns the time when the Transaction was booked.
func (t *TransactionMetadata) BookingTime() (bookingTime time.Time) {
	t.RLock()
	defer t.RUnlock()

	return t.M.BookingTime
}

// SetInclusionTime sets the inclusion time of the Transaction.
func (t *TransactionMetadata) SetInclusionTime(inclusionTime time.Time) (updated bool, previousInclusionTime time.Time) {
	t.Lock()
	defer t.Unlock()

	if inclusionTime.After(t.M.InclusionTime) && !t.M.InclusionTime.IsZero() {
		return false, t.M.InclusionTime
	}

	previousInclusionTime = t.M.InclusionTime
	t.M.InclusionTime = inclusionTime
	t.SetModified()

	return true, previousInclusionTime
}

// InclusionTime returns the inclusion time of the Transaction.
func (t *TransactionMetadata) InclusionTime() (inclusionTime time.Time) {
	t.RLock()
	defer t.RUnlock()

	return t.M.InclusionTime
}

// OutputIDs returns the identifiers of the Outputs that the Transaction created.
func (t *TransactionMetadata) OutputIDs() (outputIDs utxo.OutputIDs) {
	t.RLock()
	defer t.RUnlock()

	return t.M.OutputIDs.Clone()
}

// SetOutputIDs sets the identifiers of the Outputs that the Transaction created.
func (t *TransactionMetadata) SetOutputIDs(outputIDs utxo.OutputIDs) (modified bool) {
	t.RLock()
	defer t.RUnlock()

	if t.M.OutputIDs.Equal(outputIDs) {
		return false
	}

	t.M.OutputIDs = outputIDs
	t.SetModified()

	return true
}

// ConfirmationState returns the confirmation status of the Transaction.
func (t *TransactionMetadata) ConfirmationState() (confirmationState confirmation.State) {
	t.RLock()
	defer t.RUnlock()

	return t.M.ConfirmationState
}

// SetConfirmationState sets the confirmation status of the Transaction.
func (t *TransactionMetadata) SetConfirmationState(confirmationState confirmation.State) (modified bool) {
	t.Lock()
	defer t.Unlock()

	if t.M.ConfirmationState == confirmationState {
		return
	}

	t.M.ConfirmationState = confirmationState
	t.M.ConfirmationStateTime = clock.SyncedTime()
	t.SetModified()

	return true
}

// ConfirmationStateTime returns the last time the ConfirmationState was updated.
func (t *TransactionMetadata) ConfirmationStateTime() (confirmationStateTime time.Time) {
	t.RLock()
	defer t.RUnlock()

	return t.M.ConfirmationStateTime
}

// IsConflicting returns true if the Transaction is conflicting with another Transaction (is a Branch).
func (t *TransactionMetadata) IsConflicting() (isConflicting bool) {
	return t.BranchIDs().Is(t.ID())
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputMetadata ///////////////////////////////////////////////////////////////////////////////////////////////

// OutputMetadata represents a container for additional information about an Output.
type OutputMetadata struct {
	model.Storable[utxo.OutputID, OutputMetadata, *OutputMetadata, outputMetadata] `serix:"0"`
}

type outputMetadata struct {
	// ConsensusManaPledgeID contains the identifier of the node that received the consensus mana pledge.
	ConsensusManaPledgeID identity.ID `serix:"0"`

	// AccessManaPledgeID contains the identifier of the node that received the access mana pledge.
	AccessManaPledgeID identity.ID `serix:"1"`

	// CreationTime contains the time when the Output was created.
	CreationTime time.Time `serix:"2"`

	// BranchIDs contains the conflicting BranchIDs that this Output depends on.
	BranchIDs *set.AdvancedSet[utxo.TransactionID] `serix:"3"`

	// FirstConsumer contains the first Transaction that ever spent the Output.
	FirstConsumer utxo.TransactionID `serix:"4"`

	// FirstConsumerForked contains a boolean flag that indicates if the FirstConsumer was forked.
	FirstConsumerForked bool `serix:"5"`

	// ConfirmationState contains the confirmation status of the Output.
	ConfirmationState confirmation.State `serix:"6"`

	// ConfirmationStateTime contains the last time the ConfirmationState was updated.
	ConfirmationStateTime time.Time `serix:"7"`
}

// NewOutputMetadata returns new OutputMetadata for the given OutputID.
func NewOutputMetadata(outputID utxo.OutputID) (new *OutputMetadata) {
	new = model.NewStorable[utxo.OutputID, OutputMetadata](&outputMetadata{
		BranchIDs: utxo.NewTransactionIDs(),
	})
	new.SetID(outputID)

	return new
}

// ConsensusManaPledgeID returns the identifier of the node that received the consensus mana pledge.
func (o *OutputMetadata) ConsensusManaPledgeID() (id identity.ID) {
	o.RLock()
	defer o.RUnlock()

	return o.M.ConsensusManaPledgeID
}

// SetConsensusManaPledgeID sets the identifier of the node that received the consensus mana pledge.
func (o *OutputMetadata) SetConsensusManaPledgeID(id identity.ID) (updated bool) {
	o.Lock()
	defer o.Unlock()

	if o.M.ConsensusManaPledgeID == id {
		return false
	}

	o.M.ConsensusManaPledgeID = id
	o.SetModified()

	return true
}

// AccessManaPledgeID returns the identifier of the node that received the access mana pledge.
func (o *OutputMetadata) AccessManaPledgeID() (id identity.ID) {
	o.RLock()
	defer o.RUnlock()

	return o.M.AccessManaPledgeID
}

// SetAccessManaPledgeID sets the identifier of the node that received the access mana pledge.
func (o *OutputMetadata) SetAccessManaPledgeID(id identity.ID) (updated bool) {
	o.Lock()
	defer o.Unlock()

	if o.M.AccessManaPledgeID == id {
		return false
	}

	o.M.AccessManaPledgeID = id
	o.SetModified()

	return true
}

// CreationTime returns the creation time of the Output.
func (o *OutputMetadata) CreationTime() (creationTime time.Time) {
	o.RLock()
	defer o.RUnlock()

	return o.M.CreationTime
}

// SetCreationTime sets the creation time of the Output.
func (o *OutputMetadata) SetCreationTime(creationTime time.Time) (updated bool) {
	o.Lock()
	defer o.Unlock()

	if o.M.CreationTime == creationTime {
		return false
	}

	o.M.CreationTime = creationTime
	o.SetModified()

	return true
}

// BranchIDs returns the conflicting BranchIDs that the Output depends on.
func (o *OutputMetadata) BranchIDs() (branchIDs *set.AdvancedSet[utxo.TransactionID]) {
	o.RLock()
	defer o.RUnlock()

	return o.M.BranchIDs.Clone()
}

// SetBranchIDs sets the conflicting BranchIDs that this Transaction depends on.
func (o *OutputMetadata) SetBranchIDs(branchIDs *set.AdvancedSet[utxo.TransactionID]) (modified bool) {
	o.Lock()
	defer o.Unlock()

	if o.M.BranchIDs.Equal(branchIDs) {
		return false
	}

	o.M.BranchIDs = branchIDs.Clone()
	o.SetModified()

	return true
}

// FirstConsumer returns the first Transaction that ever spent the Output.
func (o *OutputMetadata) FirstConsumer() (firstConsumer utxo.TransactionID) {
	o.RLock()
	defer o.RUnlock()

	return o.M.FirstConsumer
}

// RegisterBookedConsumer registers a booked consumer and checks if it is conflicting with another consumer that wasn't
// forked, yet.
func (o *OutputMetadata) RegisterBookedConsumer(consumer utxo.TransactionID) (isConflicting bool, consumerToFork utxo.TransactionID) {
	o.Lock()
	defer o.Unlock()

	if o.M.FirstConsumer == utxo.EmptyTransactionID {
		o.M.FirstConsumer = consumer
		o.SetModified()

		return false, utxo.EmptyTransactionID
	}

	if o.M.FirstConsumerForked {
		return true, utxo.EmptyTransactionID
	}

	return true, o.M.FirstConsumer
}

// ConfirmationState returns the confirmation status of the Output.
func (o *OutputMetadata) ConfirmationState() (confirmationState confirmation.State) {
	o.RLock()
	defer o.RUnlock()

	return o.M.ConfirmationState
}

// SetConfirmationState sets the confirmation status of the Output.
func (o *OutputMetadata) SetConfirmationState(confirmationState confirmation.State) (modified bool) {
	o.Lock()
	defer o.Unlock()

	if o.M.ConfirmationState == confirmationState {
		return false
	}

	o.M.ConfirmationState = confirmationState
	o.M.ConfirmationStateTime = clock.SyncedTime()
	o.SetModified()

	return true
}

// ConfirmationStateTime returns the last time the ConfirmationState was updated.
func (o *OutputMetadata) ConfirmationStateTime() (confirmationState time.Time) {
	o.RLock()
	defer o.RUnlock()

	return o.M.ConfirmationStateTime
}

// IsSpent returns true if the Output has been spent.
func (o *OutputMetadata) IsSpent() (isSpent bool) {
	o.RLock()
	defer o.RUnlock()

	return o.M.FirstConsumer != utxo.EmptyTransactionID
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region OutputsMetadata //////////////////////////////////////////////////////////////////////////////////////////////

// OutputsMetadata represents a collection of OutputMetadata objects indexed by their OutputID.
type OutputsMetadata struct {
	// OrderedMap is the underlying data structure that holds the OutputMetadata objects.
	orderedmap.OrderedMap[utxo.OutputID, *OutputMetadata] `serix:"0"`
}

// NewOutputsMetadata returns a new OutputMetadata collection with the given elements.
func NewOutputsMetadata(outputsMetadata ...*OutputMetadata) (new *OutputsMetadata) {
	new = &OutputsMetadata{*orderedmap.New[utxo.OutputID, *OutputMetadata]()}
	for _, outputMeta := range outputsMetadata {
		new.Set(outputMeta.ID(), outputMeta)
	}

	return new
}

// Get returns the OutputMetadata object for the given OutputID.
func (o *OutputsMetadata) Get(id utxo.OutputID) (outputMetadata *OutputMetadata, exists bool) {
	return o.OrderedMap.Get(id)
}

// Add adds the given OutputMetadata object to the collection.
func (o *OutputsMetadata) Add(output *OutputMetadata) (added bool) {
	return o.Set(output.ID(), output)
}

func (o *OutputsMetadata) Filter(predicate func(outputMetadata *OutputMetadata) bool) (filtered *OutputsMetadata) {
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
func (o *OutputsMetadata) IDs() (ids utxo.OutputIDs) {
	ids = utxo.NewOutputIDs()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		ids.Add(outputMetadata.ID())
		return nil
	})

	return ids
}

// BranchIDs returns a union of all BranchIDs of the contained OutputMetadata objects.
func (o *OutputsMetadata) BranchIDs() (branchIDs *set.AdvancedSet[utxo.TransactionID]) {
	branchIDs = set.NewAdvancedSet[utxo.TransactionID]()
	_ = o.ForEach(func(outputMetadata *OutputMetadata) (err error) {
		branchIDs.AddAll(outputMetadata.BranchIDs())
		return nil
	})

	return branchIDs
}

// ForEach executes the callback for each element in the collection (it aborts if the callback returns an error).
func (o *OutputsMetadata) ForEach(callback func(outputMetadata *OutputMetadata) error) (err error) {
	o.OrderedMap.ForEach(func(_ utxo.OutputID, outputMetadata *OutputMetadata) bool {
		if err = callback(outputMetadata); err != nil {
			return false
		}

		return true
	})

	return err
}

// String returns a human-readable version of the OutputsMetadata.
func (o *OutputsMetadata) String() (humanReadable string) {
	structBuilder := stringify.StructBuilder("OutputsMetadata")
	_ = o.ForEach(func(outputMetadata *OutputMetadata) error {
		structBuilder.AddField(stringify.StructField(outputMetadata.ID().String(), outputMetadata))
		return nil
	})

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the reference between an Output and its spending Transaction.
type Consumer struct {
	model.StorableReferenceWithMetadata[Consumer, *Consumer, utxo.OutputID, utxo.TransactionID, consumer] `serix:"0"`
}

type consumer struct {
	// Booked contains a boolean flag that indicates whether the Consumer was completely Booked.
	Booked bool `serix:"0"`
}

// NewConsumer return a new Consumer reference from the named Output to the named Transaction.
func NewConsumer(consumedInput utxo.OutputID, transactionID utxo.TransactionID) (new *Consumer) {
	return model.NewStorableReferenceWithMetadata[Consumer](consumedInput, transactionID, &consumer{})
}

// ConsumedInput returns the identifier of the Output that was spent.
func (c *Consumer) ConsumedInput() (outputID utxo.OutputID) {
	return c.SourceID()
}

// TransactionID returns the identifier of the spending Transaction.
func (c *Consumer) TransactionID() (spendingTransaction utxo.TransactionID) {
	return c.TargetID()
}

// IsBooked returns a boolean flag that indicates whether the Consumer was completely booked.
func (c *Consumer) IsBooked() (processed bool) {
	c.RLock()
	defer c.RUnlock()

	return c.M.Booked
}

// SetBooked sets a boolean flag that indicates whether the Consumer was completely booked.
func (c *Consumer) SetBooked() (updated bool) {
	c.Lock()
	defer c.Unlock()

	if c.M.Booked {
		return
	}

	c.M.Booked = true
	c.SetModified()
	updated = true

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region EpochDiffs ///////////////////////////////////////////////////////////////////////////////////////////////////

type EpochDiffs struct {
	orderedmap.OrderedMap[epoch.Index, *OutputWithMetadata] `serix:"0"`
}

func (e *EpochDiffs) String() string {
	structBuilder := stringify.StructBuilder("EpochDiffs")
	e.OrderedMap.ForEach(func(ei epoch.Index, epochDiff *OutputWithMetadata) bool {
		structBuilder.AddField(stringify.StructField(ei.String(), epochDiff))
		return true
	})

	return structBuilder.String()
}

type OutputWithMetadata struct {
	model.Storable[utxo.OutputID, OutputWithMetadata, *OutputWithMetadata, outputWithMetadataModel] `serix:"0"`
}

type outputWithMetadataModel struct {
	OutputID       utxo.OutputID   `serix:"0"`
	Output         utxo.Output     `serix:"1"`
	OutputMetadata *OutputMetadata `serix:"2"`
}

func (o *OutputWithMetadata) String() string {
	structBuilder := stringify.StructBuilder("OutputWithMetadata")
	structBuilder.AddField(stringify.StructField("OutputID", o.ID()))
	structBuilder.AddField(stringify.StructField("Output", o.Output()))
	structBuilder.AddField(stringify.StructField("OutputMetadata", o.OutputMetadata()))

	return structBuilder.String()
}

func NewOutputWithMetadata(outputID utxo.OutputID, output utxo.Output, outputMetadata *OutputMetadata) (new *OutputWithMetadata) {
	new = model.NewStorable[utxo.OutputID, OutputWithMetadata](&outputWithMetadataModel{
		OutputID:       outputID,
		Output:         output,
		OutputMetadata: outputMetadata,
	})
	new.SetID(outputID)
	return
}

// FromObjectStorage creates an OutputWithMetadata from sequences of key and bytes.
func (o *OutputWithMetadata) FromObjectStorage(key, value []byte) error {
	err := o.Storable.FromObjectStorage(key, value)
	o.M.Output.SetID(o.ID())
	o.M.OutputMetadata.SetID(o.ID())

	return err
}

// FromBytes unmarshals an OutputWithMetadata from a sequence of bytes.
func (o *OutputWithMetadata) FromBytes(data []byte) error {
	err := o.Storable.FromBytes(data)
	o.M.Output.SetID(o.ID())
	o.M.OutputMetadata.SetID(o.ID())

	return err
}

func (o *OutputWithMetadata) Output() (output utxo.Output) {
	o.RLock()
	defer o.RUnlock()

	return o.M.Output
}

func (o *OutputWithMetadata) SetOutput(output utxo.Output) {
	o.Lock()
	defer o.Unlock()

	o.M.Output = output
	o.SetModified()

	return
}

func (o *OutputWithMetadata) OutputMetadata() (outputMetadata *OutputMetadata) {
	o.RLock()
	defer o.RUnlock()

	return o.M.OutputMetadata
}

func (o *OutputWithMetadata) SetOutputMetadata(outputMetadata *OutputMetadata) {
	o.Lock()
	defer o.Unlock()

	o.M.OutputMetadata = outputMetadata
	o.SetModified()

	return
}

type EpochDiff struct {
	model.Immutable[EpochDiff, *EpochDiff, epochDiffModel] `serix:"0"`
}

type epochDiffModel struct {
	Spent   []*OutputWithMetadata `serix:"0"`
	Created []*OutputWithMetadata `serix:"1"`
}

func NewEpochDiff(spent []*OutputWithMetadata, created []*OutputWithMetadata) (new *EpochDiff) {
	return model.NewImmutable[EpochDiff](&epochDiffModel{
		Spent:   spent,
		Created: created,
	})
}

func (e *EpochDiff) Spent() []*OutputWithMetadata {
	return e.M.Spent
}

func (e *EpochDiff) Created() []*OutputWithMetadata {
	return e.M.Spent
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
