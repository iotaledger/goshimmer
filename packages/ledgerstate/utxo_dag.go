package ledgerstate

import (
	"container/list"
	"fmt"
	"strconv"
	"sync"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/syncutils"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"
	"golang.org/x/xerrors"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

// UTXODAG represents the DAG that is formed by Transactions consuming Inputs and creating Outputs. It forms the core of
// the ledger state and keeps track of the balances and the different perceptions of potential conflicts.
type UTXODAG struct {
	Events *UTXODAGEvents

	transactionStorage          *objectstorage.ObjectStorage
	transactionMetadataStorage  *objectstorage.ObjectStorage
	outputStorage               *objectstorage.ObjectStorage
	outputMetadataStorage       *objectstorage.ObjectStorage
	consumerStorage             *objectstorage.ObjectStorage
	addressOutputMappingStorage *objectstorage.ObjectStorage
	branchDAG                   *BranchDAG
}

// NewUTXODAG create a new UTXODAG from the given details.
func NewUTXODAG(store kvstore.KVStore, branchDAG *BranchDAG) (utxoDAG *UTXODAG) {
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	utxoDAG = &UTXODAG{
		Events:                      NewUTXODAGEvents(),
		transactionStorage:          osFactory.New(PrefixTransactionStorage, TransactionFromObjectStorage, transactionStorageOptions...),
		transactionMetadataStorage:  osFactory.New(PrefixTransactionMetadataStorage, TransactionMetadataFromObjectStorage, transactionMetadataStorageOptions...),
		outputStorage:               osFactory.New(PrefixOutputStorage, OutputFromObjectStorage, outputStorageOptions...),
		outputMetadataStorage:       osFactory.New(PrefixOutputMetadataStorage, OutputMetadataFromObjectStorage, outputMetadataStorageOptions...),
		consumerStorage:             osFactory.New(PrefixConsumerStorage, ConsumerFromObjectStorage, consumerStorageOptions...),
		addressOutputMappingStorage: osFactory.New(PrefixAddressOutputMappingStorage, AddressOutputMappingFromObjectStorage, addressOutputMappingStorageOptions...),
		branchDAG:                   branchDAG,
	}
	return
}

// inputsInvalid checks if any of the Inputs is booked into the InvalidBranch.
func (u *UTXODAG) inputsInvalid(inputsMetadata []*OutputMetadata) (invalid bool) {
	for _, inputMetadata := range inputsMetadata {
		if invalid = inputMetadata.BranchID() == InvalidBranchID; invalid {
			return
		}
	}

	return
}

// inputsRejected checks if any of the Inputs is booked into a rejected Branch.
func (u *UTXODAG) inputsRejected(inputsMetadata []*OutputMetadata) (rejected bool, targetBranchID BranchID) {
	seenBranchIDs := set.New()
	for _, inputMetadata := range inputsMetadata {
		if targetBranchID = inputMetadata.BranchID(); !seenBranchIDs.Add(targetBranchID) {
			continue
		}

		cachedBranch := u.branchDAG.Branch(targetBranchID)
		branch := cachedBranch.Unwrap()

		if branch == nil {
			cachedBranch.Release()
			panic(fmt.Sprintf("failed to load Branch with %s", targetBranchID))
		}

		if rejected = branch.InclusionState() == Rejected; rejected {
			cachedBranch.Release()
			return
		}

		cachedBranch.Release()
	}

	return
}

func (u *UTXODAG) lazyBookConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata []*OutputMetadata) (err error) {
	cachedConflictBranch, _, conflictBranchErr := u.branchDAG.CreateConflictBranch(NewBranchID(transaction.ID()), NewBranchIDs(LazyBookedConflictsBranchID), nil)
	if conflictBranchErr != nil {
		err = xerrors.Errorf("failed to create ConflictBranch for lazy booked Transaction with %s: %w", transaction.ID(), conflictBranchErr)
		return
	}
	defer cachedConflictBranch.Release()

	conflictBranch := cachedConflictBranch.Unwrap()
	if conflictBranch == nil {
		err = xerrors.Errorf("failed to unwrap ConflictBranch with %s: %w", cachedConflictBranch.ID(), cerrors.ErrFatal)
		return
	}
	conflictBranch.SetLiked(false)
	conflictBranch.SetFinalized(true)

	u.lazyBookRejectedTransaction(transaction, transactionMetadata, inputsMetadata, conflictBranch.ID())

	return
}

func (u *UTXODAG) lazyBookRejectedTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata []*OutputMetadata, targetBranchID BranchID) {
	transactionMetadata.SetBranchID(targetBranchID)
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetLazyBooked(true)

	u.bookConsumers(inputsMetadata, transaction.ID(), types.Maybe)
	u.bookOutputs(transaction, targetBranchID)
}

// bookInvalidTransaction books the given Transaction into the Branch identified by InvalidBranchID.
func (u *UTXODAG) bookInvalidTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata []*OutputMetadata) {
	transactionMetadata.SetBranchID(InvalidBranchID)
	transactionMetadata.SetSolid(true)
	transactionMetadata.SetFinalized(true)

	u.bookConsumers(inputsMetadata, transaction.ID(), types.False)
	u.bookOutputs(transaction, InvalidBranchID)
}

func (u *UTXODAG) bookNonConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata []*OutputMetadata, normalizedBranchIDs BranchIDs) (err error) {
	cachedAggregatedBranch, _, err := u.branchDAG.aggregateNormalizedBranches(normalizedBranchIDs)
	if err != nil {
		err = xerrors.Errorf("failed to aggregate Branches when booking Transaction with %s: %w", transaction.ID(), err)
		return
	}
	defer cachedAggregatedBranch.Release()

	transactionMetadata.SetBranchID(cachedAggregatedBranch.ID())
	transactionMetadata.SetSolid(true)

	u.bookConsumers(inputsMetadata, transaction.ID(), types.True)
	u.bookOutputs(transaction, cachedAggregatedBranch.ID())

	return
}

// bookConsumers creates the Consumers reference between an Output and its spending Transaction. It increases the
// ConsumerCount if the Transaction is a valid spend.
func (u *UTXODAG) bookConsumers(inputsMetadata []*OutputMetadata, transactionID TransactionID, valid types.TriBool) {
	for _, inputMetadata := range inputsMetadata {
		if valid == types.True {
			inputMetadata.RegisterConsumer(transactionID)
		}

		newConsumer := NewConsumer(inputMetadata.ID(), transactionID, valid)
		if !(&CachedConsumer{CachedObject: u.consumerStorage.ComputeIfAbsent(newConsumer.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject {
			return newConsumer
		})}).Consume(func(consumer *Consumer) {
			consumer.SetValid(valid)
		}) {
			panic("failed to update valid flag of Consumer")
		}

	}
}

func (u *UTXODAG) bookOutputs(transaction *Transaction, targetBranch BranchID) {
	for outputIndex, output := range transaction.Essence().Outputs() {
		// store Output
		output.SetID(NewOutputID(transaction.ID(), uint16(outputIndex)))
		u.outputStorage.Store(output)

		// store OutputMetadata
		metadata := NewOutputMetadata(output.ID())
		metadata.SetBranchID(targetBranch)
		metadata.SetSolid(true)
		u.outputMetadataStorage.Store(metadata).Release()
	}
}

// inputsSpentByConfirmedTransaction is an internal utility function that checks if any of the given inputs was spend by a confirmed transaction already.
func (u *UTXODAG) inputsSpentByConfirmedTransaction(inputsMetadata []*OutputMetadata) (inputsSpentByConfirmedTransaction bool, err error) {
	for _, inputMetadata := range inputsMetadata {
		if inputMetadata.ConsumerCount() >= 1 {
			cachedConsumers := u.Consumers(inputMetadata.ID())
			consumers := cachedConsumers.Unwrap()
			for _, consumer := range consumers {
				inclusionState, inclusionStateErr := u.InclusionState(consumer.TransactionID())
				if inclusionStateErr != nil {
					cachedConsumers.Release()
					err = xerrors.Errorf("failed to determine InclusionState of Transaction with %s: %w", consumer.TransactionID(), inclusionStateErr)
					return
				}

				if inclusionState == Confirmed {
					cachedConsumers.Release()
					inputsSpentByConfirmedTransaction = true
					return
				}
			}
			cachedConsumers.Release()
		}
	}

	return
}

func (u *UTXODAG) determineBookingDetails(inputsMetadata []*OutputMetadata) (branchesOfInputsConflicting bool, normalizedBranchIDs BranchIDs, conflictingInputs []ConflictID, err error) {
	conflictingInputs = make([]ConflictID, 0)
	consumedBranches := make([]BranchID, len(inputsMetadata))
	for i, inputMetadata := range inputsMetadata {
		consumedBranches[i] = inputMetadata.BranchID()

		if inputMetadata.ConsumerCount() >= 1 {
			conflictingInputs = append(conflictingInputs, NewConflictID(inputMetadata.ID()))
		}
	}

	normalizedBranchIDs, err = u.branchDAG.normalizeBranches(NewBranchIDs(consumedBranches...))
	if err != nil {
		if xerrors.Is(err, ErrInvalidStateTransition) {
			branchesOfInputsConflicting = true
			err = nil
			return
		}

		err = xerrors.Errorf("failed to normalize branches: %w", cerrors.ErrFatal)
		return
	}

	return
}

// BookTransaction books a Transaction into the ledger state.
func (u *UTXODAG) BookTransaction(transaction *Transaction) (err error) {
	cachedInputs := u.transactionInputs(transaction)
	defer cachedInputs.Release()
	inputs := cachedInputs.Unwrap()

	// perform cheap checks
	if !u.inputsSolid(inputs) {
		err = xerrors.Errorf("not all transactionInputs of transaction are solid: %w", ErrTransactionNotSolid)
		return
	}
	if !u.transactionBalancesValid(inputs, transaction.Essence().Outputs()) {
		err = xerrors.Errorf("sum of consumed and spent balances is not 0: %w", ErrTransactionInvalid)
		return
	}
	if !u.unlockBlocksValid(inputs, transaction) {
		err = xerrors.Errorf("spending of referenced transactionInputs is not authorized: %w", ErrTransactionInvalid)
		return
	}

	// use TransactionMetadata as a synchronization mechanism to only book every Transaction once
	transactionMetadata := NewTransactionMetadata(transaction.ID())
	transactionMetadata.SetSolid(true)
	cachedTransactionMetadata, stored := u.transactionMetadataStorage.StoreIfAbsent(transactionMetadata)
	if !stored {
		return
	}
	defer cachedTransactionMetadata.Release()
	u.transactionStorage.Store(transaction).Release()

	// retrieve the metadata of the Inputs
	cachedInputsMetadata := u.transactionInputsMetadata(transaction)
	defer cachedInputsMetadata.Release()
	inputsMetadata := cachedInputsMetadata.Unwrap()

	// check if Transaction is attaching to something invalid
	if invalid := u.inputsInvalid(inputsMetadata); invalid {
		u.bookInvalidTransaction(transaction, transactionMetadata, inputsMetadata)
	}

	// check if transaction is attaching to something rejected
	if rejected, targetBranch := u.inputsRejected(inputsMetadata); rejected {
		u.lazyBookRejectedTransaction(transaction, transactionMetadata, inputsMetadata, targetBranch)
	}

	// check if any Input was spent by a confirmed Transaction already
	if inputsSpentByConfirmedTransaction, tmpErr := u.inputsSpentByConfirmedTransaction(inputsMetadata); tmpErr != nil {
		err = xerrors.Errorf("failed to check if inputs were spent by confirmed Transaction: %w", err)
		return
	} else if inputsSpentByConfirmedTransaction {
		err = u.lazyBookConflictingTransaction(transaction, transactionMetadata, inputsMetadata)
	}

	// mark transaction as "permanently rejected"
	if !u.inputsPastConeValid(inputs, inputsMetadata) {
		u.bookInvalidTransaction(transaction, transactionMetadata, inputsMetadata)
		return
	}

	// determine the booking details before we book
	branchesOfInputsConflicting, normalizedBranchIDs, conflictingInputs, err := u.determineBookingDetails(inputsMetadata)
	if err != nil {
		err = xerrors.Errorf("failed to determine book details of Transaction with %s: %w", transaction.ID(), err)
		return
	}

	// are branches of inputs conflicting
	if branchesOfInputsConflicting {
		u.bookInvalidTransaction(transaction, transactionMetadata, inputsMetadata)
		return
	}

	if len(conflictingInputs) >= 1 {
		conflictIDs := NewConflictIDs(conflictingInputs...)
		// book into ConflictBranch + Fork existing consumers
	}

	if err = u.bookNonConflictingTransaction(transaction, transactionMetadata, inputsMetadata, normalizedBranchIDs); err != nil {
		err = xerrors.Errorf("failed to book non-conflicting Transaction with %s: %w", transaction.ID(), err)
		return
	}

	return
}

func (u *UTXODAG) InclusionState(transactionID TransactionID) (inclusionState InclusionState, err error) {
	cachedTransactionMetadata := u.TransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()
	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		err = xerrors.Errorf("failed to load TransactionMetadata with %s: %w", transactionID, cerrors.ErrFatal)
		return
	}

	cachedBranch := u.branchDAG.Branch(transactionMetadata.BranchID())
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = xerrors.Errorf("failed to load Branch with %s: %w", transactionMetadata.BranchID(), cerrors.ErrFatal)
		return
	}

	if branch.InclusionState() != Confirmed {
		inclusionState = branch.InclusionState()
		return
	}

	if transactionMetadata.Finalized() {
		inclusionState = Confirmed
		return
	}

	inclusionState = Pending
	return
}

// Transaction retrieves the Transaction with the given TransactionID from the object storage.
func (u *UTXODAG) Transaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return &CachedTransaction{CachedObject: u.transactionStorage.Load(transactionID.Bytes())}
}

// TransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (u *UTXODAG) TransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return &CachedTransactionMetadata{CachedObject: u.transactionMetadataStorage.Load(transactionID.Bytes())}
}

// Output retrieves the Output with the given OutputID from the object storage.
func (u *UTXODAG) Output(outputID OutputID) (cachedOutput *CachedOutput) {
	return &CachedOutput{CachedObject: u.outputStorage.Load(outputID.Bytes())}
}

// OutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (u *UTXODAG) OutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata) {
	return &CachedOutputMetadata{CachedObject: u.outputMetadataStorage.Load(outputID.Bytes())}
}

// Consumers retrieves the Consumers of the given OutputID from the object storage.
func (u *UTXODAG) Consumers(outputID OutputID) (cachedConsumers CachedConsumers) {
	cachedConsumers = make(CachedConsumers, 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConsumers = append(cachedConsumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputID.Bytes())

	return
}

// transactionInputs is an internal utility function that returns the Outputs that are used as Inputs by the given
// Transaction.
func (u *UTXODAG) transactionInputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	cachedInputs = make(CachedOutputs, 0)
	for _, input := range transaction.Essence().Inputs() {
		switch input.Type() {
		case UTXOInputType:
			cachedInputs = append(cachedInputs, u.Output(input.(*UTXOInput).ReferencedOutputID()))
		default:
			panic(fmt.Sprintf("unsupported InputType: %s", input.Type()))
		}
	}

	return
}

// transactionInputsMetadata is an internal utility function that returns the Metadata of the Outputs that are used as
// Inputs by the given Transaction.
func (u *UTXODAG) transactionInputsMetadata(transaction *Transaction) (cachedInputsMetadata CachedOutputsMetadata) {
	cachedInputsMetadata = make(CachedOutputsMetadata, 0)
	for _, inputMetadata := range transaction.Essence().Inputs() {
		switch inputMetadata.Type() {
		case UTXOInputType:
			cachedInputsMetadata = append(cachedInputsMetadata, u.OutputMetadata(inputMetadata.(*UTXOInput).ReferencedOutputID()))
		default:
			panic(fmt.Sprintf("unsupported InputType: %s", inputMetadata.Type()))
		}
	}

	return
}

// inputsSolid is an internal utility function that checks if all of the given Inputs exist.
func (u *UTXODAG) inputsSolid(inputs []Output) (solid bool) {
	for _, input := range inputs {
		if typeutils.IsInterfaceNil(input) {
			// TODO: TRIGGER EVENT FOR EVENTUAL SOLIDIFICATION
			return false
		}
	}

	return true
}

func (u *UTXODAG) transactionBalancesValid(inputs []Output, outputs []Output) (valid bool) {
	// sum up the balances
	consumedBalances := make(map[Color]uint64)
	for _, input := range inputs {
		input.Balances().ForEach(func(color Color, balance uint64) bool {
			consumedBalances[color] += balance

			return true
		})
	}
	for _, output := range outputs {
		output.Balances().ForEach(func(color Color, balance uint64) bool {
			consumedBalances[color] -= balance

			return true
		})
	}

	// check if the balances are all 0
	for _, remainingBalance := range consumedBalances {
		if remainingBalance != 0 {
			return false
		}
	}

	return true
}

func (u *UTXODAG) unlockBlocksValid(inputs []Output, transaction *Transaction) (valid bool) {
	unlockBlocks := transaction.UnlockBlocks()
	for i, input := range inputs {
		unlockValid, unlockErr := input.UnlockValid(transaction, unlockBlocks[i])
		if !unlockValid || unlockErr != nil {
			return false
		}
	}

	return true
}

func (u *UTXODAG) outputsUnspent(inputsMetadata []*OutputMetadata) (outputsUnspent bool) {
	for _, inputMetadata := range inputsMetadata {
		if inputMetadata.ConsumerCount() != 0 {
			return false
		}
	}

	return true
}

// inputsPastConeValid is an internal utility function that checks if the given Inputs do not reference their own past
// cone.
func (u *UTXODAG) inputsPastConeValid(inputs []Output, inputsMetadata []*OutputMetadata) (pastConeValid bool) {
	if u.outputsUnspent(inputsMetadata) {
		pastConeValid = true
		return
	}

	stack := list.New()
	consumedInputIDs := make(map[OutputID]types.Empty)
	for _, input := range inputs {
		consumedInputIDs[input.ID()] = types.Void
		stack.PushBack(input.ID())
	}

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		cachedConsumers := u.Consumers(firstElement.Value.(OutputID))
		for _, consumer := range cachedConsumers.Unwrap() {
			if consumer == nil {
				cachedConsumers.Release()
				panic("failed to unwrap Consumer")
			}

			cachedTransaction := u.Transaction(consumer.TransactionID())
			transaction := cachedTransaction.Unwrap()
			if transaction == nil {
				cachedTransaction.Release()
				cachedConsumers.Release()
				panic("failed to unwrap Transaction")
			}

			for _, output := range transaction.Essence().Outputs() {
				if _, exists := consumedInputIDs[output.ID()]; exists {
					cachedTransaction.Release()
					cachedConsumers.Release()
					return false
				}

				stack.PushBack(output.ID())
			}

			cachedTransaction.Release()
		}
		cachedConsumers.Release()
	}

	return true
}

// TODO: IMPLEMENT A GOOD SYNCHRONIZATION MECHANISM FOR THE UTXODAG
func (u *UTXODAG) lockTransaction(transaction *Transaction) {
	var lockBuilder syncutils.MultiMutexLockBuilder
	for _, input := range transaction.Essence().Inputs() {
		lockBuilder.AddLock(input.(*UTXOInput).ReferencedOutputID())
	}
	for outputIndex := range transaction.Essence().Outputs() {
		lockBuilder.AddLock(NewOutputID(transaction.ID(), uint16(outputIndex)))
	}
	var mutex syncutils.RWMultiMutex
	mutex.Lock(lockBuilder.Build()...)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXODAGEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// UTXODAGEvents is a container for all of the UTXODAG related events.
type UTXODAGEvents struct {
}

// NewUTXODAGEvents creates a container for all of the UTXODAG related events.
func NewUTXODAGEvents() *UTXODAGEvents {
	return &UTXODAGEvents{}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

// AddressOutputMapping represents a mapping between Addresses and their corresponding Outputs. Since an Address can have a
// potentially unbounded amount of Outputs, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type AddressOutputMapping struct {
	address  Address
	outputID OutputID

	objectstorage.StorableObjectFlags
}

// AddressOutputMappingFromBytes unmarshals a AddressOutputMapping from a sequence of bytes.
func AddressOutputMappingFromBytes(bytes []byte) (addressOutputMapping *AddressOutputMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if addressOutputMapping, err = AddressOutputMappingFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse AddressOutputMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AddressOutputMappingFromMarshalUtil unmarshals an AddressOutputMapping using a MarshalUtil (for easier unmarshaling).
func AddressOutputMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (addressOutputMapping *AddressOutputMapping, err error) {
	addressOutputMapping = &AddressOutputMapping{}
	if addressOutputMapping.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse consumed Address from MarshalUtil: %w", err)
		return
	}
	if addressOutputMapping.outputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
		return
	}

	return
}

// AddressOutputMappingFromObjectStorage is a factory method that creates a new AddressOutputMapping instance from a
// storage key of the object storage. It is used by the object storage, to create new instances of this entity.
func AddressOutputMappingFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = AddressOutputMappingFromBytes(key); err != nil {
		err = xerrors.Errorf("failed to parse AddressOutputMapping from bytes: %w", err)
		return
	}

	return
}

// Address returns the Address of the AddressOutputMapping.
func (a *AddressOutputMapping) Address() Address {
	return a.address
}

// OutputID returns the OutputID of the AddressOutputMapping.
func (a *AddressOutputMapping) OutputID() OutputID {
	return a.outputID
}

// Bytes marshals the Consumer into a sequence of bytes.
func (a *AddressOutputMapping) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns a human readable version of the Consumer.
func (a *AddressOutputMapping) String() (humanReadableConsumer string) {
	return stringify.Struct("AddressOutputMapping",
		stringify.StructField("address", a.address),
		stringify.StructField("outputID", a.outputID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (a *AddressOutputMapping) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *AddressOutputMapping) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(a.address.Bytes(), a.outputID.Bytes())
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (a *AddressOutputMapping) ObjectStorageValue() []byte {
	panic("implement me")
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = &AddressOutputMapping{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedAddressOutputMapping ///////////////////////////////////////////////////////////////////////////////////

// CachedAddressOutputMapping is a wrapper for the generic CachedObject returned by the object storage that overrides
// the accessor methods with a type-casted one.
type CachedAddressOutputMapping struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedAddressOutputMapping) Retain() *CachedAddressOutputMapping {
	return &CachedAddressOutputMapping{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedAddressOutputMapping) Unwrap() *AddressOutputMapping {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*AddressOutputMapping)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedAddressOutputMapping) Consume(consumer func(addressOutputMapping *AddressOutputMapping), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*AddressOutputMapping))
	}, forceRelease...)
}

// String returns a human readable version of the CachedAddressOutputMapping.
func (c *CachedAddressOutputMapping) String() string {
	return stringify.Struct("CachedAddressOutputMapping",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedAddressOutputMappings //////////////////////////////////////////////////////////////////////////////////

// CachedAddressOutputMappings represents a collection of CachedAddressOutputMapping objects.
type CachedAddressOutputMappings []*CachedAddressOutputMapping

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedAddressOutputMappings) Unwrap() (unwrappedOutputs []*AddressOutputMapping) {
	unwrappedOutputs = make([]*AddressOutputMapping, len(c))
	for i, cachedAddressOutputMapping := range c {
		untypedObject := cachedAddressOutputMapping.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*AddressOutputMapping)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedOutputs[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedAddressOutputMappings) Consume(consumer func(addressOutputMapping *AddressOutputMapping), forceRelease ...bool) (consumed bool) {
	for _, cachedAddressOutputMapping := range c {
		consumed = cachedAddressOutputMapping.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedAddressOutputMappings) Release(force ...bool) {
	for _, cachedAddressOutputMapping := range c {
		cachedAddressOutputMapping.Release(force...)
	}
}

// String returns a human readable version of the CachedAddressOutputMappings.
func (c CachedAddressOutputMappings) String() string {
	structBuilder := stringify.StructBuilder("CachedAddressOutputMappings")
	for i, cachedAddressOutputMapping := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedAddressOutputMapping))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	consumedInput OutputID
	transactionID TransactionID
	validMutex    sync.RWMutex
	valid         types.TriBool

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput OutputID, transactionID TransactionID, valid types.TriBool) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
		valid:         valid,
	}
}

// ConsumerFromBytes unmarshals a Consumer from a sequence of bytes.
func ConsumerFromBytes(bytes []byte) (consumer *Consumer, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if consumer, err = ConsumerFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse Consumer from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConsumerFromMarshalUtil unmarshals an Consumer using a MarshalUtil (for easier unmarshaling).
func ConsumerFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (consumer *Consumer, err error) {
	consumer = &Consumer{}
	if consumer.consumedInput, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
		return
	}
	if consumer.transactionID, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	if consumer.valid, err = types.TriBoolFromMarshalUtil(marshalUtil); err != nil {
		err = xerrors.Errorf("failed to parse valid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ConsumerFromObjectStorage is a factory method that creates a new Consumer instance from a storage key of the
// object storage. It is used by the object storage, to create new instances of this entity.
func ConsumerFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ConsumerFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = xerrors.Errorf("failed to parse Consumer from bytes: %w", err)
		return
	}

	return
}

// ConsumedInput returns the OutputID of the consumed Input.
func (c *Consumer) ConsumedInput() OutputID {
	return c.consumedInput
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() TransactionID {
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

// String returns a human readable version of the Consumer.
func (c *Consumer) String() (humanReadableConsumer string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("transactionID", c.transactionID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *Consumer) Update(other objectstorage.StorableObject) {
	panic("updates disabled")
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
var _ objectstorage.StorableObject = &Consumer{}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumer ///////////////////////////////////////////////////////////////////////////////////////////////

// CachedConsumer is a wrapper for the generic CachedObject returned by the object storage that overrides the accessor
// methods with a type-casted one.
type CachedConsumer struct {
	objectstorage.CachedObject
}

// Retain marks the CachedObject to still be in use by the program.
func (c *CachedConsumer) Retain() *CachedConsumer {
	return &CachedConsumer{c.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (c *CachedConsumer) Unwrap() *Consumer {
	untypedObject := c.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Consumer)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (c *CachedConsumer) Consume(consumer func(consumer *Consumer), forceRelease ...bool) (consumed bool) {
	return c.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Consumer))
	}, forceRelease...)
}

// String returns a human readable version of the CachedConsumer.
func (c *CachedConsumer) String() string {
	return stringify.Struct("CachedConsumer",
		stringify.StructField("CachedObject", c.Unwrap()),
	)
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region CachedConsumers //////////////////////////////////////////////////////////////////////////////////////////////

// CachedConsumers represents a collection of CachedConsumer objects.
type CachedConsumers []*CachedConsumer

// Unwrap is the type-casted equivalent of Get. It returns a slice of unwrapped objects with the object being nil if it
// does not exist.
func (c CachedConsumers) Unwrap() (unwrappedOutputs []*Consumer) {
	unwrappedOutputs = make([]*Consumer, len(c))
	for i, cachedConsumer := range c {
		untypedObject := cachedConsumer.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*Consumer)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedOutputs[i] = typedObject
	}

	return
}

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (c CachedConsumers) Consume(consumer func(consumer *Consumer), forceRelease ...bool) (consumed bool) {
	for _, cachedConsumer := range c {
		consumed = cachedConsumer.Consume(consumer, forceRelease...) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (c CachedConsumers) Release(force ...bool) {
	for _, cachedConsumer := range c {
		cachedConsumer.Release(force...)
	}
}

// String returns a human readable version of the CachedConsumers.
func (c CachedConsumers) String() string {
	structBuilder := stringify.StructBuilder("CachedConsumers")
	for i, cachedConsumer := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedConsumer))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
