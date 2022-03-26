package old

import (
	"container/list"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/refactored/txvm"
	"github.com/iotaledger/goshimmer/packages/refactored/utxo"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

// UTXODAG represents the DAG that is formed by Transactions consuming Inputs and creating Outputs. It forms the core of
// the ledger state and keeps track of the balances and the different perceptions of potential conflicts.
type UTXODAG struct {
	events *UTXODAGEvents

	ledgerstate *Ledger
	vm          utxo.VM

	addressOutputMappingStorage *objectstorage.ObjectStorage[*AddressOutputMapping]
	shutdownOnce                sync.Once
}

// NewUTXODAG create a new UTXODAG from the given details.
func NewUTXODAG(ledgerstate *Ledger, vm utxo.VM) (utxoDAG *UTXODAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)
	utxoDAG = &UTXODAG{
		events: &UTXODAGEvents{
			TransactionBranchIDUpdatedByFork: events.NewEvent(TransactionBranchIDUpdatedByForkEventHandler),
		},
		ledgerstate:                 ledgerstate,
		vm:                          vm,
		transactionStorage:          objectstorage.New[utxo.Transaction](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionStorage}), options.transactionStorageOptions...),
		transactionMetadataStorage:  objectstorage.New[*TransactionMetadata](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixTransactionMetadataStorage}), options.transactionMetadataStorageOptions...),
		outputStorage:               objectstorage.New[utxo.Output](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputStorage}), options.outputStorageOptions...),
		outputMetadataStorage:       objectstorage.New[*OutputMetadata](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixOutputMetadataStorage}), options.outputMetadataStorageOptions...),
		consumerStorage:             objectstorage.New[*Consumer](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixConsumerStorage}), options.consumerStorageOptions...),
		addressOutputMappingStorage: objectstorage.New[*AddressOutputMapping](ledgerstate.Options.Store.WithRealm([]byte{database.PrefixLedger, PrefixAddressOutputMappingStorage}), options.addressOutputMappingStorageOptions...),
	}
	return
}

// Events returns all events of the UTXODAG.
func (u *UTXODAG) Events() *UTXODAGEvents {
	return u.events
}

// Shutdown shuts down the UTXODAG and persists its state.
func (u *UTXODAG) Shutdown() {
	u.shutdownOnce.Do(func() {
		u.transactionStorage.Shutdown()
		u.transactionMetadataStorage.Shutdown()
		u.outputStorage.Shutdown()
		u.outputMetadataStorage.Shutdown()
		u.consumerStorage.Shutdown()
		u.addressOutputMappingStorage.Shutdown()
	})
}

// CheckTransaction contains fast checks that have to be performed before booking a Transaction.
func (u *UTXODAG) CheckTransaction(transaction utxo.Transaction) (err error) {
	inputs, allAvailable, err := u.vm.ResolveInput(transaction.Inputs()...)
	if err != nil {
		return errors.Errorf("failed to resolve inputs of Transaction with %s: %w", transaction.ID(), cerrors.ErrFatal)
	}
	if !allAvailable {
		return errors.Errorf("not all inputs of transaction are solid: %w", ErrTransactionNotSolid)
	}

	// retrieve the metadata of the Inputs
	cachedInputsMetadata := u.outputsMetadata(inputs)
	defer cachedInputsMetadata.Release()
	inputsMetadata := cachedInputsMetadata.Unwrap()

	// mark transaction as "permanently rejected"
	if !u.consumedOutputsPastConeValid(consumedOutputs, inputsMetadata) {
		return errors.Errorf("consumed outputs reference each other: %w", ErrTransactionInvalid)
	}

	outputs, err := u.vm.ExecuteTransaction(transaction, inputs, 0)
	if err != nil {
		return err
	}

	return nil
}

// TransactionBranchIDs returns the BranchIDs of the given Transaction.
func (u *UTXODAG) TransactionBranchIDs(transactionID TransactionID) (branchIDs branchdag.BranchIDs, err error) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		branchIDs = transactionMetadata.BranchIDs()
	}) {
		err = errors.Errorf("failed to retrieve TransactionMetadata for Transaction with %s: %w", transactionID, err)
	}

	return
}

// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
func (u *UTXODAG) ConflictingTransactions(transaction *Transaction) (conflictingTransactions TransactionIDs) {
	conflictingTransactions = make(TransactionIDs)
	for _, input := range transaction.Essence().Inputs() {
		u.CachedConsumers(input.(*UTXOInput).ReferencedOutputID()).Consume(func(consumer *Consumer) {
			if consumer.TransactionID() == transaction.ID() {
				return
			}

			conflictingTransactions[consumer.TransactionID()] = types.Void
		})
	}
	return
}

// TransactionGradeOfFinality returns the GradeOfFinality of the Transaction with the given TransactionID.
func (u *UTXODAG) TransactionGradeOfFinality(transactionID TransactionID) (gradeOfFinality gof.GradeOfFinality, err error) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		gradeOfFinality = transactionMetadata.GradeOfFinality()
	}) {
		return gof.None, errors.Errorf("failed to load TransactionMetadata with %s: %w", transactionID, cerrors.ErrFatal)
	}

	return
}

// BranchGradeOfFinality returns the GradeOfFinality of the Branch with the given BranchID.
func (u *UTXODAG) BranchGradeOfFinality(branchID branchdag.BranchID) (gradeOfFinality gof.GradeOfFinality, err error) {
	if branchID == branchdag.MasterBranchID {
		return gof.High, nil
	}

	branchGof, gofErr := u.TransactionGradeOfFinality(branchID.TransactionID())
	if gofErr != nil {
		return gof.None, errors.Errorf("failed to normalize %s: %w", branchID, err)
	}

	return branchGof, nil
}

// Transaction returns a specific transaction, consumed.
func (u *UTXODAG) Transaction(transactionID TransactionID) (transaction *Transaction) {
	u.CachedTransaction(transactionID).Consume(func(tx *Transaction) {
		transaction = tx
	})
	return transaction
}

// Transactions returns all the transactions, consumed.
func (u *UTXODAG) Transactions() (transactions map[TransactionID]*Transaction) {
	transactions = make(map[TransactionID]*Transaction)
	u.transactionStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Transaction]) bool {
		cachedObject.Consume(func(transaction *Transaction) {
			transactions[transaction.ID()] = transaction
		})
		return true
	})
	return
}

// LoadSnapshot creates a set of outputs in the UTXODAG, that are forming the genesis for future transactions.
func (u *UTXODAG) LoadSnapshot(snapshot *Snapshot) {
	for txID, record := range snapshot.Transactions {
		transaction := NewTransaction(record.Essence, record.UnlockBlocks)
		cached, storedTx := u.transactionStorage.StoreIfAbsent(transaction)

		if storedTx {
			cached.Release()
		}

		for i, output := range record.Essence.outputs {
			if !record.UnspentOutputs[i] {
				continue
			}
			cachedOutput, stored := u.outputStorage.StoreIfAbsent(output)
			if stored {
				cachedOutput.Release()
			}

			// store addressOutputMapping
			u.ManageStoreAddressOutputMapping(output)

			// store OutputMetadata
			metadata := NewOutputMetadata(output.ID())
			metadata.AddBranchID(branchdag.MasterBranchID)
			metadata.SetSolid(true)
			metadata.SetGradeOfFinality(gof.High)
			cachedMetadata, stored := u.outputMetadataStorage.StoreIfAbsent(metadata)
			if stored {
				cachedMetadata.Release()
			}
		}

		// store TransactionMetadata
		txMetadata := NewTransactionMetadata(txID)
		txMetadata.SetSolid(true)
		txMetadata.AddBranchID(branchdag.MasterBranchID)
		txMetadata.SetGradeOfFinality(gof.High)

		u.transactionMetadataStorage.ComputeIfAbsent(txID.Bytes(), func(key []byte) *TransactionMetadata {
			txMetadata.Persist()
			txMetadata.SetModified()
			return txMetadata
		}).Release()
	}
}

// CachedAddressOutputMapping retrieves the outputs for the given address.
func (u *UTXODAG) CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings objectstorage.CachedObjects[*AddressOutputMapping]) {
	u.addressOutputMappingStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*AddressOutputMapping]) bool {
		cachedAddressOutputMappings = append(cachedAddressOutputMappings, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(address.Bytes()))
	return
}

// region booking functions ////////////////////////////////////////////////////////////////////////////////////////////

// forkConsumer is an internal utility function that creates a Branch for a Transaction that has not been
// conflicting first but now turned out to be conflicting because of a newly booked double spend.
func (u *UTXODAG) forkConsumer(transactionID TransactionID, conflictingInputs OutputsMetadataByID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		forkedBranchID := branchdag.NewBranchID(transactionID)
		conflictIDs := conflictingInputs.Filter(u.consumedOutputIDsOfTransaction(transactionID)).ConflictIDs()

		cachedConsumingBranch, _, err := u.ledgerstate.CreateBranch(forkedBranchID, transactionMetadata.BranchIDs(), conflictIDs)
		if err != nil {
			panic(fmt.Errorf("failed to create Branch when forking Transaction with %s: %w", transactionID, err))
		}
		cachedConsumingBranch.Release()

		// We don't need to propagate updates if the branch did already exist.
		// Though CreateBranch needs to be called so that conflict sets and conflict membership are properly updated.
		if transactionMetadata.BranchIDs().Is(forkedBranchID) {
			return
		}

		// Because we are forking the transaction, automatically all the outputs and the transaction itself need to go
		// into the newly forked branch (own branch) and override all other existing branches. These are now mapped via
		// the BranchDAG (parents of forked branch).
		forkedBranchIDs := branchdag.NewBranchIDs(forkedBranchID)
		outputIds := u.createdOutputIDsOfTransaction(transactionID)
		for _, outputID := range outputIds {
			if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
				outputMetadata.SetBranchIDs(forkedBranchIDs)
			}) {
				panic("failed to load OutputMetadata")
			}
		}

		transactionMetadata.SetBranchIDs(forkedBranchIDs)
		u.Events().TransactionBranchIDUpdatedByFork.Trigger(&TransactionBranchIDUpdatedByForkEvent{
			TransactionID:  transactionID,
			ForkedBranchID: forkedBranchID,
		})

		u.walkFutureCone(outputIds, func(transactionID TransactionID) (updatedOutputs []OutputID) {
			return u.propagateBranch(transactionID, forkedBranchID)
		}, types.True)
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}
}

// propagateBranch is an internal utility function that propagates changes in the perception of the BranchDAG
// after introducing a new Branch.
func (u *UTXODAG) propagateBranch(transactionID TransactionID, forkedBranchID branchdag.BranchID) (updatedOutputs []OutputID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		if transactionMetadata.IsConflicting() {
			for transactionBranchID := range transactionMetadata.BranchIDs() {
				if err := u.ledgerstate.AddBranchParent(transactionBranchID, forkedBranchID); err != nil {
					panic(fmt.Errorf("failed to update Branch with %s: %w", transactionBranchID, err))
				}
			}

			return
		}

		if transactionMetadata.AddBranchID(forkedBranchID) {
			updatedOutputs = u.createdOutputIDsOfTransaction(transactionID)
			for _, outputID := range updatedOutputs {
				if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
					outputMetadata.AddBranchID(forkedBranchID)
				}) {
					panic(fmt.Errorf("failed to load OutputMetadata with %s", outputID))
				}
			}

			u.Events().TransactionBranchIDUpdatedByFork.Trigger(&TransactionBranchIDUpdatedByForkEvent{
				TransactionID:  transactionID,
				ForkedBranchID: forkedBranchID,
			})
		}
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region private utility functions ////////////////////////////////////////////////////////////////////////////////////

// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
func (u *UTXODAG) ConsumedOutputs(transaction *Transaction) (cachedInputs objectstorage.CachedObjects[Output]) {
	cachedInputs = make(objectstorage.CachedObjects[Output], 0)
	for _, input := range transaction.Essence().Inputs() {
		cachedInputs = append(cachedInputs, u.CachedOutput(input.(*UTXOInput).ReferencedOutputID()))
	}

	return
}

// outputsMetadata is an internal utility function that returns the Metadata of the Outputs that are used as
// Inputs by the given Transaction.
func (u *UTXODAG) outputsMetadata(outputs []utxo.Output) (cachedOutputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedOutputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)
	for _, output := range outputs {
		cachedOutputsMetadata = append(cachedOutputsMetadata, u.CachedOutputMetadata(output.ID()))
	}

	return
}

// consumedOutputsPastConeValid is an internal utility function that checks if the given Outputs do not directly or
// indirectly reference each other in their own past cone.
func (u *UTXODAG) consumedOutputsPastConeValid(outputs []utxo.Output, outputsMetadata OutputsMetadata) (pastConeValid bool) {
	if u.outputsUnspent(outputsMetadata) {
		pastConeValid = true
		return
	}

	stack := list.New()
	consumedInputIDs := make(map[OutputID]types.Empty)
	for _, input := range outputs {
		consumedInputIDs[input.ID()] = types.Void
		stack.PushBack(input.ID())
	}

	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		cachedConsumers := u.CachedConsumers(firstElement.Value.(OutputID))
		for _, consumer := range cachedConsumers.Unwrap() {
			if consumer == nil {
				cachedConsumers.Release()
				panic("failed to unwrap Consumer")
			}

			cachedTransaction := u.CachedTransaction(consumer.TransactionID())
			transaction, exists := cachedTransaction.Unwrap()
			if !exists {
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

// outputsUnspent is an internal utility function that checks if the given outputs are unspent (do not have a valid
// Consumer, yet).
func (u *UTXODAG) outputsUnspent(outputsMetadata OutputsMetadata) (outputsUnspent bool) {
	for _, inputMetadata := range outputsMetadata {
		if inputMetadata.ConsumerCount() != 0 {
			return false
		}
	}

	return true
}

// createdOutputIDsOfTransaction is an internal utility function that returns the list of OutputIDs that were created by
// the given Transaction. If the Transaction can not be found, it returns an empty list.
func (u *UTXODAG) createdOutputIDsOfTransaction(transactionID TransactionID) (outputIDs []OutputID) {
	outputIDs = make([]OutputID, 0)
	u.CachedTransaction(transactionID).Consume(func(transaction *Transaction) {
		for index := range transaction.Essence().Outputs() {
			outputIDs = append(outputIDs, NewOutputID(transactionID, uint16(index)))
		}
	})

	return
}

// walkFutureCone is an internal utility function that walks through the future cone of the given Outputs and calling
// the callback function on each step. It is possible to provide an optional filter for the valid flag of the Consumer
// to only walk through matching Consumers.
func (u *UTXODAG) walkFutureCone(entryPoints []OutputID, callback func(transactionID TransactionID) (nextOutputsToVisit []OutputID), optionalValidFlagFilter ...types.TriBool) {
	stack := list.New()
	for _, outputID := range entryPoints {
		stack.PushBack(outputID)
	}

	seenTransactions := set.New[TransactionID]()
	for stack.Len() > 0 {
		firstElement := stack.Front()
		stack.Remove(firstElement)

		u.CachedConsumers(firstElement.Value.(OutputID)).Consume(func(consumer *Consumer) {
			if !seenTransactions.Add(consumer.TransactionID()) {
				return
			}

			if len(optionalValidFlagFilter) >= 1 && consumer.Valid() != optionalValidFlagFilter[0] {
				return
			}

			for _, updatedOutputID := range callback(consumer.TransactionID()) {
				stack.PushBack(updatedOutputID)
			}
		})
	}
}

// ManageStoreAddressOutputMapping manages how to store the address-output mapping dependent on which type of output it is.
func (u *UTXODAG) ManageStoreAddressOutputMapping(output Output) {
	switch output.Type() {
	case AliasOutputType:
		castedOutput := output.(*AliasOutput)
		// if it is an origin alias output, we don't have the AliasAddress from the parsed bytes.
		// that happens in utxodag output booking, so we calculate the alias address here
		u.StoreAddressOutputMapping(castedOutput.GetAliasAddress(), output.ID())
		u.StoreAddressOutputMapping(castedOutput.GetStateAddress(), output.ID())
		if !castedOutput.IsSelfGoverned() {
			u.StoreAddressOutputMapping(castedOutput.GetGoverningAddress(), output.ID())
		}
	case ExtendedLockedOutputType:
		castedOutput := output.(*ExtendedLockedOutput)
		if castedOutput.FallbackAddress() != nil {
			u.StoreAddressOutputMapping(castedOutput.FallbackAddress(), output.ID())
		}
		u.StoreAddressOutputMapping(output.Address(), output.ID())
	default:
		u.StoreAddressOutputMapping(output.Address(), output.ID())
	}
}

// StoreAddressOutputMapping stores the address-output mapping.
func (u *UTXODAG) StoreAddressOutputMapping(address Address, outputID OutputID) {
	result, stored := u.addressOutputMappingStorage.StoreIfAbsent(NewAddressOutputMapping(address, outputID))
	if stored {
		result.Release()
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXODAGEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// UTXODAGEvents is a container for all the UTXODAG related events.
type UTXODAGEvents struct {
	// TransactionBranchIDUpdatedByFork gets triggered when the BranchID of a Transaction is changed after the initial booking.
	TransactionBranchIDUpdatedByFork *events.Event
}

// TransactionIDEventHandler is an event handler for an event with a TransactionID.
func TransactionIDEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(utxo.TransactionID))(params[0].(utxo.TransactionID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionBranchIDUpdatedByForkEvent ////////////////////////////////////////////////////////////////////////

// TransactionBranchIDUpdatedByForkEvent is an event that gets triggered, whenever the BranchID of a Transaction is
// changed.
type TransactionBranchIDUpdatedByForkEvent struct {
	TransactionID  utxo.TransactionID
	ForkedBranchID branchdag.BranchID
}

// TransactionBranchIDUpdatedByForkEventHandler is an event handler for an event with a
// TransactionBranchIDUpdatedByForkEvent.
func TransactionBranchIDUpdatedByForkEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(*TransactionBranchIDUpdatedByForkEvent))(params[0].(*TransactionBranchIDUpdatedByForkEvent))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region AddressOutputMapping /////////////////////////////////////////////////////////////////////////////////////////

// AddressOutputMapping represents a mapping between Addresses and their corresponding Outputs. Since an Address can have a
// potentially unbounded amount of Outputs, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the OutputEssence.
type AddressOutputMapping struct {
	address  txvm.Address
	outputID OutputID

	objectstorage.StorableObjectFlags
}

// NewAddressOutputMapping returns a new AddressOutputMapping.
func NewAddressOutputMapping(address Address, outputID OutputID) *AddressOutputMapping {
	return &AddressOutputMapping{
		address:  address,
		outputID: outputID,
	}
}

// FromObjectStorage creates an TransactionMetadata from sequences of key and bytes.
func (a *AddressOutputMapping) FromObjectStorage(key, _ []byte) (objectstorage.StorableObject, error) {
	result, err := a.FromBytes(key)
	if err != nil {
		err = errors.Errorf("failed to parse AddressOutputMapping from bytes: %w", err)
	}
	return result, err
}

// FromBytes unmarshals a AddressOutputMapping from a sequence of bytes.
func (a *AddressOutputMapping) FromBytes(bytes []byte) (addressOutputMapping objectstorage.StorableObject, err error) {
	marshalUtil := marshalutil.New(bytes)
	if addressOutputMapping, err = a.FromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse AddressOutputMapping from MarshalUtil: %w", err)
		return
	}
	return
}

// FromMarshalUtil unmarshals an AddressOutputMapping using a MarshalUtil (for easier unmarshalling).
func (a *AddressOutputMapping) FromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (addressOutputMapping *AddressOutputMapping, err error) {
	if addressOutputMapping = a; addressOutputMapping == nil {
		addressOutputMapping = new(AddressOutputMapping)
	}
	if addressOutputMapping.address, err = AddressFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse consumed Address from MarshalUtil: %w", err)
		return
	}
	if addressOutputMapping.outputID, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse OutputID from MarshalUtil: %w", err)
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

// String returns a human-readable version of the Consumer.
func (a *AddressOutputMapping) String() (humanReadableConsumer string) {
	return stringify.Struct("AddressOutputMapping",
		stringify.StructField("address", a.address),
		stringify.StructField("outputID", a.outputID),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *AddressOutputMapping) ObjectStorageKey() []byte {
	return byteutils.ConcatBytes(a.address.Bytes(), a.outputID.Bytes())
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (a *AddressOutputMapping) ObjectStorageValue() (value []byte) {
	return
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(AddressOutputMapping)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
