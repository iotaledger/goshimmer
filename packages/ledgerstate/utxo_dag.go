package ledgerstate

import (
	"container/list"
	"context"
	"fmt"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/serix"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/database"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

// IUTXODAG is the interface for UTXODAG which is the core of the ledger state
// that is formed by Transactions consuming Inputs and creating Outputs.  It represents all the methods
// that helps to keep track of the balances and the different perceptions of potential Conflicts.
type IUTXODAG interface {
	// Events returns all events of the UTXODAG
	Events() *UTXODAGEvents
	// Shutdown shuts down the UTXODAG and persists its state.
	Shutdown()
	// CheckTransaction contains fast checks that have to be performed before booking a Transaction.
	CheckTransaction(transaction *Transaction) (err error)
	// BookTransaction books a Transaction into the ledger state.
	BookTransaction(transaction *Transaction) (targetBranch BranchID, err error)
	// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
	CachedTransaction(transactionID TransactionID) (cachedTransaction *objectstorage.CachedObject[*Transaction])
	// Transaction returns a specific transaction, consumed.
	Transaction(transactionID TransactionID) (transaction *Transaction)
	// Transactions returns all the transactions, consumed.
	Transactions() (transactions map[TransactionID]*Transaction)
	// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
	CachedTransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata])
	// CachedOutput retrieves the Output with the given OutputID from the object storage.
	CachedOutput(outputID OutputID) (cachedOutput *objectstorage.CachedObject[Output])
	// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
	CachedOutputMetadata(outputID OutputID) (cachedOutput *objectstorage.CachedObject[*OutputMetadata])
	// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
	CachedConsumers(outputID OutputID) (cachedConsumers *objectstorage.CachedObjects[*Consumer])
	// LoadSnapshot creates a set of outputs in the UTXODAG, that are forming the genesis for future transactions.
	LoadSnapshot(snapshot *Snapshot)
	// CachedAddressOutputMapping retrieves the outputs for the given address.
	CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings *objectstorage.CachedObject[*AddressOutputMapping])
	// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
	ConsumedOutputs(transaction *Transaction) (cachedInputs *objectstorage.CachedObjects[Output])
	// ManageStoreAddressOutputMapping manages how to store the address-output mapping dependent on which type of output it is.
	ManageStoreAddressOutputMapping(output Output)
	// StoreAddressOutputMapping stores the address-output mapping.
	StoreAddressOutputMapping(address Address, outputID OutputID)
	// TransactionGradeOfFinality returns the GradeOfFinality of the Transaction with the given TransactionID.
	TransactionGradeOfFinality(transactionID TransactionID) (gradeOfFinality gof.GradeOfFinality, err error)
	// BranchGradeOfFinality returns the GradeOfFinality of the Branch with the given BranchID.
	BranchGradeOfFinality(branchID BranchID) (gradeOfFinality gof.GradeOfFinality, err error)
	// ConflictingTransactions returns the TransactionIDs that are conflicting with the given Transaction.
	ConflictingTransactions(transaction *Transaction) (conflictingTransactions TransactionIDs)
}

// UTXODAG represents the DAG that is formed by Transactions consuming Inputs and creating Outputs. It forms the core of
// the ledger state and keeps track of the balances and the different perceptions of potential Conflicts.
type UTXODAG struct {
	events *UTXODAGEvents

	ledgerstate *Ledgerstate

	transactionStorage          *objectstorage.ObjectStorage[*Transaction]
	transactionMetadataStorage  *objectstorage.ObjectStorage[*TransactionMetadata]
	outputStorage               *objectstorage.ObjectStorage[Output]
	outputMetadataStorage       *objectstorage.ObjectStorage[*OutputMetadata]
	consumerStorage             *objectstorage.ObjectStorage[*Consumer]
	addressOutputMappingStorage *objectstorage.ObjectStorage[*AddressOutputMapping]
	shutdownOnce                sync.Once
}

// NewUTXODAG create a new UTXODAG from the given details.
func NewUTXODAG(ledgerstate *Ledgerstate) (utxoDAG *UTXODAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)

	utxoDAG = &UTXODAG{
		events: &UTXODAGEvents{
			TransactionBranchIDUpdatedByFork: events.NewEvent(TransactionBranchIDUpdatedByForkEventHandler),
		},
		ledgerstate:                 ledgerstate,
		transactionStorage:          objectstorage.New[*Transaction](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixTransactionStorage), options.transactionStorageOptions...),
		transactionMetadataStorage:  objectstorage.New[*TransactionMetadata](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixTransactionMetadataStorage), options.transactionMetadataStorageOptions...),
		outputStorage:               objectstorage.New[Output](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixOutputStorage), options.outputStorageOptions...),
		outputMetadataStorage:       objectstorage.New[*OutputMetadata](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixOutputMetadataStorage), options.outputMetadataStorageOptions...),
		consumerStorage:             objectstorage.New[*Consumer](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixConsumerStorage), options.consumerStorageOptions...),
		addressOutputMappingStorage: objectstorage.New[*AddressOutputMapping](objectstorage.NewStoreWithRealm(ledgerstate.Options.Store, database.PrefixLedgerState, PrefixAddressOutputMappingStorage), options.addressOutputMappingStorageOptions...),
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
func (u *UTXODAG) CheckTransaction(transaction *Transaction) (err error) {
	cachedConsumedOutputs := u.ConsumedOutputs(transaction)
	defer cachedConsumedOutputs.Release()
	consumedOutputs := cachedConsumedOutputs.Unwrap()

	// perform cheap checks
	if !u.allOutputsExist(consumedOutputs) {
		return errors.Errorf("not all consumedOutputs of transaction are solid: %w", ErrTransactionNotSolid)
	}
	if !TransactionBalancesValid(consumedOutputs, transaction.Essence().Outputs()) {
		return errors.Errorf("sum of consumed and spent balances is not 0: %w", ErrTransactionInvalid)
	}
	if !UnlockBlocksValid(consumedOutputs, transaction) {
		return errors.Errorf("spending of referenced consumedOutputs is not authorized: %w", ErrTransactionInvalid)
	}
	if !AliasInitialStateValid(consumedOutputs, transaction) {
		return errors.Errorf("initial state of created alias output is invalid: %w", ErrTransactionInvalid)
	}

	// retrieve the metadata of the Inputs
	cachedInputsMetadata := u.transactionInputsMetadata(transaction)
	defer cachedInputsMetadata.Release()
	inputsMetadata := cachedInputsMetadata.Unwrap()

	// mark transaction as "permanently rejected"
	if !u.consumedOutputsPastConeValid(consumedOutputs, inputsMetadata) {
		return errors.Errorf("consumed outputs reference each other: %w", ErrTransactionInvalid)
	}

	return nil
}

// BookTransaction books a Transaction into the ledger state.
func (u *UTXODAG) BookTransaction(transaction *Transaction) (targetBranchIDs BranchIDs, err error) {
	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(transaction.ID())
	transactionMetadata.SetSolid(true)
	newTransaction := false
	cachedTransactionMetadata := u.transactionMetadataStorage.ComputeIfAbsent(transaction.ID().Bytes(), func(key []byte) *TransactionMetadata {
		newTransaction = true

		transactionMetadata.Persist()
		transactionMetadata.SetModified()

		return transactionMetadata
	})
	if !newTransaction {
		if !cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			targetBranchIDs = transactionMetadata.BranchIDs()
		}) {
			err = errors.Errorf("failed to load TransactionMetadata with %s: %w", transaction.ID(), cerrors.ErrFatal)
		}
		return
	}
	defer cachedTransactionMetadata.Release()

	// store Transaction
	u.transactionStorage.Store(transaction).Release()

	// retrieve the metadata of the Inputs
	cachedInputsMetadata := u.transactionInputsMetadata(transaction)
	defer cachedInputsMetadata.Release()
	inputsMetadata := cachedInputsMetadata.Unwrap()

	// determine the booking details before we book
	parentBranchIDs, conflictingInputs, err := u.determineBookingDetails(inputsMetadata)
	if err != nil {
		err = errors.Errorf("failed to determine book details of Transaction with %s: %w", transaction.ID(), err)
		return
	}

	if len(conflictingInputs) != 0 {
		return u.bookConflictingTransaction(transaction, transactionMetadata, inputsMetadata, parentBranchIDs, conflictingInputs.ByID()), nil
	}

	return u.bookNonConflictingTransaction(transaction, transactionMetadata, inputsMetadata, parentBranchIDs), nil
}

// TransactionBranchIDs returns the BranchIDs of the given Transaction.
func (u *UTXODAG) TransactionBranchIDs(transactionID TransactionID) (branchIDs BranchIDs, err error) {
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
func (u *UTXODAG) BranchGradeOfFinality(branchID BranchID) (gradeOfFinality gof.GradeOfFinality, err error) {
	if branchID == MasterBranchID {
		return gof.High, nil
	}

	branchGof, gofErr := u.TransactionGradeOfFinality(branchID.TransactionID())
	if gofErr != nil {
		return gof.None, errors.Errorf("failed to normalize %s: %w", branchID, err)
	}

	return branchGof, nil
}

// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
func (u *UTXODAG) CachedTransaction(transactionID TransactionID) (cachedTransaction *objectstorage.CachedObject[*Transaction]) {
	return u.transactionStorage.Load(transactionID.Bytes())
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

// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (u *UTXODAG) CachedTransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *objectstorage.CachedObject[*TransactionMetadata]) {
	return u.transactionMetadataStorage.Load(transactionID.Bytes())
}

// CachedOutput retrieves the Output with the given OutputID from the object storage.
func (u *UTXODAG) CachedOutput(outputID OutputID) (cachedOutput *objectstorage.CachedObject[Output]) {
	return u.outputStorage.Load(outputID.Bytes())
}

// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (u *UTXODAG) CachedOutputMetadata(outputID OutputID) (cachedOutput *objectstorage.CachedObject[*OutputMetadata]) {
	return u.outputMetadataStorage.Load(outputID.Bytes())
}

// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
func (u *UTXODAG) CachedConsumers(outputID OutputID) (cachedConsumers objectstorage.CachedObjects[*Consumer]) {
	cachedConsumers = make(objectstorage.CachedObjects[*Consumer], 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Consumer]) bool {
		cachedConsumers = append(cachedConsumers, cachedObject)
		return true
	}, objectstorage.WithIteratorPrefix(outputID.Bytes()))

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

		for i, output := range record.Essence.Outputs() {
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
			metadata.AddBranchID(MasterBranchID)
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
		txMetadata.AddBranchID(MasterBranchID)
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

// bookNonConflictingTransaction is an internal utility function that books the Transaction into the Branch that is
// determined by aggregating the Branches of the consumed Inputs.
func (u *UTXODAG) bookNonConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, branchIDs BranchIDs) (targetBranchIDs BranchIDs) {
	transactionMetadata.SetBranchIDs(branchIDs)
	transactionMetadata.SetSolid(true)
	u.bookConsumers(inputsMetadata, transaction.ID(), types.True)
	u.bookOutputs(transaction, branchIDs)

	return branchIDs
}

// bookConflictingTransaction is an internal utility function that books a Transaction that uses Inputs that have
// already been spent by another Transaction. It creates a new Branch for the new Transaction and "forks" the
// existing consumers of the conflicting Inputs.
func (u *UTXODAG) bookConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, branchIDs BranchIDs, conflictingInputs OutputsMetadataByID) (targetBranchIDs BranchIDs) {
	// fork existing consumers
	u.walkFutureCone(conflictingInputs.IDs(), func(transactionID TransactionID) (nextOutputsToVisit []OutputID) {
		u.forkConsumer(transactionID, conflictingInputs)

		return
	}, types.True)

	// create new Branch
	targetBranchID := NewBranchID(transaction.ID())
	cachedBranch, _, err := u.ledgerstate.CreateBranch(targetBranchID, branchIDs, conflictingInputs.ConflictIDs())
	if err != nil {
		panic(fmt.Errorf("failed to create Branch when booking Transaction with %s: %w", transaction.ID(), err))
	}
	cachedBranch.Release()

	targetBranchIDs = NewBranchIDs(targetBranchID)
	transactionMetadata.SetBranchIDs(targetBranchIDs)
	transactionMetadata.SetSolid(true)
	u.bookConsumers(inputsMetadata, transaction.ID(), types.True)
	u.bookOutputs(transaction, targetBranchIDs)

	return
}

// forkConsumer is an internal utility function that creates a Branch for a Transaction that has not been
// conflicting first but now turned out to be conflicting because of a newly booked double spend.
func (u *UTXODAG) forkConsumer(transactionID TransactionID, conflictingInputs OutputsMetadataByID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		forkedBranchID := NewBranchID(transactionID)
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
		forkedBranchIDs := NewBranchIDs(forkedBranchID)
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
func (u *UTXODAG) propagateBranch(transactionID TransactionID, forkedBranchID BranchID) (updatedOutputs []OutputID) {
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

// bookConsumers creates the reference between an Output and its spending Transaction. It increases the ConsumerCount if
// the Transaction is a valid spend.
func (u *UTXODAG) bookConsumers(inputsMetadata OutputsMetadata, transactionID TransactionID, valid types.TriBool) {
	for _, inputMetadata := range inputsMetadata {
		if valid == types.True {
			inputMetadata.RegisterConsumer(transactionID)
		}

		newConsumer := NewConsumer(inputMetadata.ID(), transactionID, valid)
		if !u.consumerStorage.ComputeIfAbsent(newConsumer.ObjectStorageKey(), func(key []byte) *Consumer {
			newConsumer.Persist()
			newConsumer.SetModified()

			return newConsumer
		}).Consume(func(consumer *Consumer) {
			consumer.SetValid(valid)
		}) {
			panic("failed to update valid flag of Consumer")
		}
	}
}

// bookOutputs creates the Outputs and their corresponding OutputsMetadata in the object storage.
func (u *UTXODAG) bookOutputs(transaction *Transaction, targetBranchIDs BranchIDs) {
	for _, output := range transaction.Essence().Outputs() {
		// replace ColorMint color with unique color based on OutputID
		updatedOutput := output.UpdateMintingColor()
		// store Output
		u.outputStorage.Store(updatedOutput).Release()

		// store OutputMetadata
		metadata := NewOutputMetadata(updatedOutput.ID())
		metadata.SetBranchIDs(targetBranchIDs)
		metadata.SetSolid(true)
		u.outputMetadataStorage.Store(metadata).Release()
	}
}

// determineBookingDetails is an internal utility function that determines the information that are required to fully
// book a newly arrived Transaction into the UTXODAG using the metadata of its referenced Inputs.
func (u *UTXODAG) determineBookingDetails(inputsMetadata OutputsMetadata) (inheritedBranchIDs BranchIDs, conflictingInputs OutputsMetadata, err error) {
	conflictingInputs = inputsMetadata.SpentOutputsMetadata()
	inheritedBranchIDs = NewBranchIDs()
	for _, inputMetadata := range inputsMetadata {
		inheritedBranchIDs.AddAll(inputMetadata.BranchIDs())
	}

	inheritedBranchIDs, err = u.ledgerstate.ResolvePendingBranchIDs(inheritedBranchIDs)

	if err != nil {
		err = errors.Errorf("failed to resolve pending branches: %w", cerrors.ErrFatal)
		return
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

// allOutputsExist is an internal utility function that checks if all the given Inputs exist.
func (u *UTXODAG) allOutputsExist(inputs Outputs) (solid bool) {
	for _, input := range inputs {
		if typeutils.IsInterfaceNil(input) {
			return false
		}
	}
	return true
}

// transactionInputsMetadata is an internal utility function that returns the Metadata of the Outputs that are used as
// Inputs by the given Transaction.
func (u *UTXODAG) transactionInputsMetadata(transaction *Transaction) (cachedInputsMetadata objectstorage.CachedObjects[*OutputMetadata]) {
	cachedInputsMetadata = make(objectstorage.CachedObjects[*OutputMetadata], 0)
	for _, inputMetadata := range transaction.Essence().Inputs() {
		cachedInputsMetadata = append(cachedInputsMetadata, u.CachedOutputMetadata(inputMetadata.(*UTXOInput).ReferencedOutputID()))
	}

	return
}

// consumedOutputsPastConeValid is an internal utility function that checks if the given Outputs do not directly or
// indirectly reference each other in their own past cone.
func (u *UTXODAG) consumedOutputsPastConeValid(outputs Outputs, outputsMetadata OutputsMetadata) (pastConeValid bool) {
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

// consumedOutputIDsOfTransaction is an internal utility function returns a list of OutputIDs that were consumed by a
// given Transaction. If the Transaction can not be found, it returns an empty list.
func (u *UTXODAG) consumedOutputIDsOfTransaction(transactionID TransactionID) (inputIDs []OutputID) {
	inputIDs = make([]OutputID, 0)
	u.CachedTransaction(transactionID).Consume(func(transaction *Transaction) {
		for _, input := range transaction.Essence().Inputs() {
			inputIDs = append(inputIDs, input.(*UTXOInput).ReferencedOutputID())
		}
	})

	return
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
	handler.(func(TransactionID))(params[0].(TransactionID))
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region TransactionBranchIDUpdatedByForkEvent ////////////////////////////////////////////////////////////////////////

// TransactionBranchIDUpdatedByForkEvent is an event that gets triggered, whenever the BranchID of a Transaction is
// changed.
type TransactionBranchIDUpdatedByForkEvent struct {
	TransactionID  TransactionID
	ForkedBranchID BranchID
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
// list of spending Transactions inside the Output.
type AddressOutputMapping struct {
	addressOutputMappingInner `serix:"0"`
}
type addressOutputMappingInner struct {
	Address  Address  `serix:"0"`
	OutputID OutputID `serix:"1"`

	objectstorage.StorableObjectFlags
}

// NewAddressOutputMapping returns a new AddressOutputMapping.
func NewAddressOutputMapping(address Address, outputID OutputID) *AddressOutputMapping {
	return &AddressOutputMapping{
		addressOutputMappingInner{
			Address:  address,
			OutputID: outputID,
		},
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
func (a *AddressOutputMapping) FromBytes(data []byte) (addressOutputMapping *AddressOutputMapping, err error) {
	if addressOutputMapping = a; addressOutputMapping == nil {
		addressOutputMapping = new(AddressOutputMapping)
	}
	_, err = serix.DefaultAPI.Decode(context.Background(), data, addressOutputMapping, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse AddressOutputMapping: %w", err)
		return
	}
	return
}

// Address returns the Address of the AddressOutputMapping.
func (a *AddressOutputMapping) Address() Address {
	return a.addressOutputMappingInner.Address
}

// OutputID returns the OutputID of the AddressOutputMapping.
func (a *AddressOutputMapping) OutputID() OutputID {
	return a.addressOutputMappingInner.OutputID
}

// Bytes marshals the Consumer into a sequence of bytes.
func (a *AddressOutputMapping) Bytes() []byte {
	return a.ObjectStorageKey()
}

// String returns a human-readable version of the Consumer.
func (a *AddressOutputMapping) String() (humanReadableConsumer string) {
	return stringify.Struct("AddressOutputMapping",
		stringify.StructField("Address", a.Address()),
		stringify.StructField("OutputID", a.OutputID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (a *AddressOutputMapping) ObjectStorageKey() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), a, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (a *AddressOutputMapping) ObjectStorageValue() (value []byte) {
	return
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(AddressOutputMapping)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object storage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{OutputIDLength, TransactionIDLength}...)

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	consumerInner `serix:"0"`
}
type consumerInner struct {
	ConsumedInput OutputID
	TransactionID TransactionID
	Valid         types.TriBool `serix:"0"`
	validMutex    sync.RWMutex

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput OutputID, transactionID TransactionID, valid types.TriBool) *Consumer {
	return &Consumer{
		consumerInner{
			ConsumedInput: consumedInput,
			TransactionID: transactionID,
			Valid:         valid,
		},
	}
}

// FromObjectStorage creates an Consumer from sequences of key and bytes.
func (c *Consumer) FromObjectStorage(key, value []byte) (objectstorage.StorableObject, error) {
	result, err := c.FromBytes(byteutils.ConcatBytes(key, value))
	if err != nil {
		err = errors.Errorf("failed to parse Consumer from bytes: %w", err)
	}
	return result, err
}

// FromBytes creates an Consumer from sequences of bytes.
func (c *Consumer) FromBytes(data []byte) (consumer *Consumer, err error) {
	if consumer = c; consumer == nil {
		consumer = new(Consumer)
	}
	consumedInputID := new(OutputID)
	readBytesConsumedInput, err := serix.DefaultAPI.Decode(context.Background(), data, consumedInputID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Consumer.ConsumedInput: %w", err)
		return
	}

	transactionID := new(TransactionID)
	readBytesTransactionID, err := serix.DefaultAPI.Decode(context.Background(), data[readBytesConsumedInput:], transactionID, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Consumer.ConsumedInput: %w", err)
		return
	}
	_, err = serix.DefaultAPI.Decode(context.Background(), data[readBytesConsumedInput+readBytesTransactionID:], consumer, serix.WithValidation())
	if err != nil {
		err = errors.Errorf("failed to parse Consumer: %w", err)
		return
	}
	consumer.consumerInner.ConsumedInput = *consumedInputID
	consumer.consumerInner.TransactionID = *transactionID
	return
}

// ConsumedInput returns the OutputID of the consumed Input.
func (c *Consumer) ConsumedInput() OutputID {
	return c.consumerInner.ConsumedInput
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() TransactionID {
	return c.consumerInner.TransactionID
}

// Valid returns a flag that indicates if the spending Transaction is valid or not.
func (c *Consumer) Valid() (valid types.TriBool) {
	c.validMutex.RLock()
	defer c.validMutex.RUnlock()

	return c.consumerInner.Valid
}

// SetValid updates the valid flag of the Consumer and returns true if the value was changed.
func (c *Consumer) SetValid(valid types.TriBool) (updated bool) {
	c.validMutex.Lock()
	defer c.validMutex.Unlock()

	if valid == c.consumerInner.Valid {
		return
	}

	c.consumerInner.Valid = valid
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
		stringify.StructField("ConsumedInput", c.ConsumedInput()),
		stringify.StructField("TransactionID", c.TransactionID()),
	)
}

// ObjectStorageKey returns the key that is used to store the object in the database. It is required to match the
// StorableObject interface.
func (c *Consumer) ObjectStorageKey() []byte {
	inputBytes, err := serix.DefaultAPI.Encode(context.Background(), c.ConsumedInput(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}

	txBytes, err := serix.DefaultAPI.Encode(context.Background(), c.TransactionID(), serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}

	return byteutils.ConcatBytes(inputBytes, txBytes)
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (c *Consumer) ObjectStorageValue() []byte {
	objBytes, err := serix.DefaultAPI.Encode(context.Background(), c, serix.WithValidation())
	if err != nil {
		// TODO: what do?
		panic(err)
	}
	return objBytes
}

// code contract (make sure the struct implements all required methods)
var _ objectstorage.StorableObject = new(Consumer)

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
