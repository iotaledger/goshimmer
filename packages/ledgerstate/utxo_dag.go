package ledgerstate

import (
	"container/list"
	"fmt"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/byteutils"
	"github.com/iotaledger/hive.go/cerrors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/typeutils"

	"github.com/iotaledger/goshimmer/packages/consensus/gof"
	"github.com/iotaledger/goshimmer/packages/database"
)

// region UTXODAG //////////////////////////////////////////////////////////////////////////////////////////////////////

// IUTXODAG is the interface for UTXODAG which is the core of the ledger state
// that is formed by Transactions consuming Inputs and creating Outputs.  It represents all the methods
// that helps to keep track of the balances and the different perceptions of potential conflicts.
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
	CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction)
	// Transaction returns a specific transaction, consumed.
	Transaction(transactionID TransactionID) (transaction *Transaction)
	// Transactions returns all the transactions, consumed.
	Transactions() (transactions map[TransactionID]*Transaction)
	// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
	CachedTransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *CachedTransactionMetadata)
	// CachedOutput retrieves the Output with the given OutputID from the object storage.
	CachedOutput(outputID OutputID) (cachedOutput *CachedOutput)
	// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
	CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata)
	// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
	CachedConsumers(outputID OutputID) (cachedConsumers CachedConsumers)
	// LoadSnapshot creates a set of outputs in the UTXODAG, that are forming the genesis for future transactions.
	LoadSnapshot(snapshot *Snapshot)
	// CachedAddressOutputMapping retrieves the outputs for the given address.
	CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings)
	// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
	ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs)
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
// the ledger state and keeps track of the balances and the different perceptions of potential conflicts.
type UTXODAG struct {
	events *UTXODAGEvents

	ledgerstate *Ledgerstate

	transactionStorage          *objectstorage.ObjectStorage
	transactionMetadataStorage  *objectstorage.ObjectStorage
	outputStorage               *objectstorage.ObjectStorage
	outputMetadataStorage       *objectstorage.ObjectStorage
	consumerStorage             *objectstorage.ObjectStorage
	addressOutputMappingStorage *objectstorage.ObjectStorage
	shutdownOnce                sync.Once
}

// NewUTXODAG create a new UTXODAG from the given details.
func NewUTXODAG(ledgerstate *Ledgerstate) (utxoDAG *UTXODAG) {
	options := buildObjectStorageOptions(ledgerstate.Options.CacheTimeProvider)
	osFactory := objectstorage.NewFactory(ledgerstate.Options.Store, database.PrefixLedgerState)
	utxoDAG = &UTXODAG{
		events: &UTXODAGEvents{
			TransactionBranchIDUpdatedByFork: events.NewEvent(TransactionBranchIDUpdatedByForkEventHandler),
		},
		ledgerstate:                 ledgerstate,
		transactionStorage:          osFactory.New(PrefixTransactionStorage, TransactionFromObjectStorage, options.transactionStorageOptions...),
		transactionMetadataStorage:  osFactory.New(PrefixTransactionMetadataStorage, TransactionMetadataFromObjectStorage, options.transactionMetadataStorageOptions...),
		outputStorage:               osFactory.New(PrefixOutputStorage, OutputFromObjectStorage, options.outputStorageOptions...),
		outputMetadataStorage:       osFactory.New(PrefixOutputMetadataStorage, OutputMetadataFromObjectStorage, options.outputMetadataStorageOptions...),
		consumerStorage:             osFactory.New(PrefixConsumerStorage, ConsumerFromObjectStorage, options.consumerStorageOptions...),
		addressOutputMappingStorage: osFactory.New(PrefixAddressOutputMappingStorage, AddressOutputMappingFromObjectStorage, options.addressOutputMappingStorageOptions...),
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
func (u *UTXODAG) BookTransaction(transaction *Transaction) (targetBranch BranchID, err error) {
	// store TransactionMetadata
	transactionMetadata := NewTransactionMetadata(transaction.ID())
	transactionMetadata.SetSolid(true)
	newTransaction := false
	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: u.transactionMetadataStorage.ComputeIfAbsent(transaction.ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		newTransaction = true

		transactionMetadata.Persist()
		transactionMetadata.SetModified()

		return transactionMetadata
	})}
	if !newTransaction {
		if !cachedTransactionMetadata.Consume(func(transactionMetadata *TransactionMetadata) {
			targetBranch = transactionMetadata.BranchID()
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
		if branchIDs, err = u.ledgerstate.ResolveConflictBranchIDs(NewBranchIDs(transactionMetadata.BranchID())); err != nil {
			err = errors.Errorf("failed to resolve ConflictBranchIDs of Transaction with %s: %w", transactionID, err)
		}
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

	resolvedConflictBranchIDs, err := u.ledgerstate.ResolveConflictBranchIDs(NewBranchIDs(branchID))
	if err != nil {
		return gof.None, errors.Errorf("failed to normalize %s: %w", branchID, err)
	}

	gradeOfFinality = gof.High
	for conflictBranchID := range resolvedConflictBranchIDs {
		conflictBranchGoF, gofErr := u.TransactionGradeOfFinality(conflictBranchID.TransactionID())
		if gofErr != nil {
			return gof.None, errors.Errorf("failed to normalize %s: %w", branchID, err)
		}

		if conflictBranchGoF < gradeOfFinality {
			gradeOfFinality = conflictBranchGoF
		}
	}

	return gradeOfFinality, nil
}

// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
func (u *UTXODAG) CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction) {
	return &CachedTransaction{CachedObject: u.transactionStorage.Load(transactionID.Bytes())}
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
	u.transactionStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		(&CachedTransaction{CachedObject: cachedObject}).Consume(func(transaction *Transaction) {
			transactions[transaction.ID()] = transaction
		})
		return true
	})
	return
}

// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
func (u *UTXODAG) CachedTransactionMetadata(transactionID TransactionID) (cachedTransactionMetadata *CachedTransactionMetadata) {
	return &CachedTransactionMetadata{CachedObject: u.transactionMetadataStorage.Load(transactionID.Bytes())}
}

// CachedOutput retrieves the Output with the given OutputID from the object storage.
func (u *UTXODAG) CachedOutput(outputID OutputID) (cachedOutput *CachedOutput) {
	return &CachedOutput{CachedObject: u.outputStorage.Load(outputID.Bytes())}
}

// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
func (u *UTXODAG) CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata) {
	return &CachedOutputMetadata{CachedObject: u.outputMetadataStorage.Load(outputID.Bytes())}
}

// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
func (u *UTXODAG) CachedConsumers(outputID OutputID) (cachedConsumers CachedConsumers) {
	cachedConsumers = make(CachedConsumers, 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConsumers = append(cachedConsumers, &CachedConsumer{CachedObject: cachedObject})

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
			metadata.SetBranchID(MasterBranchID)
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
		txMetadata.SetBranchID(MasterBranchID)
		txMetadata.SetGradeOfFinality(gof.High)

		(&CachedTransactionMetadata{CachedObject: u.transactionMetadataStorage.ComputeIfAbsent(txID.Bytes(), func(key []byte) objectstorage.StorableObject {
			txMetadata.Persist()
			txMetadata.SetModified()
			return txMetadata
		})}).Release()
	}
}

// CachedAddressOutputMapping retrieves the outputs for the given address.
func (u *UTXODAG) CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings) {
	u.addressOutputMappingStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedAddressOutputMappings = append(cachedAddressOutputMappings, &CachedAddressOutputMapping{cachedObject})
		return true
	}, objectstorage.WithIteratorPrefix(address.Bytes()))
	return
}

// region booking functions ////////////////////////////////////////////////////////////////////////////////////////////

// bookNonConflictingTransaction is an internal utility function that books the Transaction into the Branch that is
// determined by aggregating the Branches of the consumed Inputs.
func (u *UTXODAG) bookNonConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, normalizedBranchIDs BranchIDs) (targetBranch BranchID) {
	targetBranch = u.ledgerstate.AggregateConflictBranchesID(normalizedBranchIDs)

	transactionMetadata.SetBranchID(targetBranch)
	transactionMetadata.SetSolid(true)
	u.bookConsumers(inputsMetadata, transaction.ID(), types.True)
	u.bookOutputs(transaction, targetBranch)

	return
}

// bookConflictingTransaction is an internal utility function that books a Transaction that uses Inputs that have
// already been spent by another Transaction. It creates a new ConflictBranch for the new Transaction and "forks" the
// existing consumers of the conflicting Inputs.
func (u *UTXODAG) bookConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, normalizedBranchIDs BranchIDs, conflictingInputs OutputsMetadataByID) (targetBranch BranchID) {
	// fork existing consumers
	u.walkFutureCone(conflictingInputs.IDs(), func(transactionID TransactionID) (nextOutputsToVisit []OutputID) {
		u.forkConsumer(transactionID, conflictingInputs)

		return
	}, types.True)

	// create new ConflictBranch
	targetBranch = NewBranchID(transaction.ID())
	cachedConflictBranch, _, err := u.ledgerstate.CreateConflictBranch(targetBranch, normalizedBranchIDs, conflictingInputs.ConflictIDs())
	if err != nil {
		panic(fmt.Errorf("failed to create ConflictBranch when booking Transaction with %s: %w", transaction.ID(), err))
	}

	// book Transaction into new ConflictBranch
	if !cachedConflictBranch.Consume(func(branch Branch) {
		transactionMetadata.SetBranchID(targetBranch)
		transactionMetadata.SetSolid(true)
		u.bookConsumers(inputsMetadata, transaction.ID(), types.True)
		u.bookOutputs(transaction, targetBranch)
	}) {
		panic(fmt.Errorf("failed to load ConflictBranch with %s", cachedConflictBranch.ID()))
	}

	return
}

// forkConsumer is an internal utility function that creates a ConflictBranch for a Transaction that has not been
// conflicting first but now turned out to be conflicting because of a newly booked double spend.
func (u *UTXODAG) forkConsumer(transactionID TransactionID, conflictingInputs OutputsMetadataByID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(txMetadata *TransactionMetadata) {
		conflictBranchID := NewBranchID(transactionID)
		conflictBranchParents := NewBranchIDs(txMetadata.BranchID())
		conflictIDs := conflictingInputs.Filter(u.consumedOutputIDsOfTransaction(transactionID)).ConflictIDs()

		cachedConsumingConflictBranch, _, err := u.ledgerstate.CreateConflictBranch(conflictBranchID, conflictBranchParents, conflictIDs)
		if err != nil {
			panic(fmt.Errorf("failed to create ConflictBranch when forking Transaction with %s: %w", transactionID, err))
		}
		cachedConsumingConflictBranch.Release()

		// We don't need to propagate updates if the branch did already exist.
		// Though CreateConflictBranch needs to be called so that conflict sets and conflict membership are properly updated.
		if txMetadata.BranchID() == conflictBranchID {
			return
		}

		outputIds := u.createdOutputIDsOfTransaction(transactionID)
		for _, outputID := range outputIds {
			if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
				outputMetadata.SetBranchID(conflictBranchID)
			}) {
				panic("failed to load OutputMetadata")
			}
		}

		txMetadata.SetBranchID(conflictBranchID)
		u.Events().TransactionBranchIDUpdatedByFork.Trigger(&TransactionBranchIDUpdatedByForkEvent{
			TransactionID:  transactionID,
			NewBranchID:    conflictBranchID,
			ForkedBranchID: conflictBranchID,
		})

		u.walkFutureCone(outputIds, func(transactionID TransactionID) (updatedOutputs []OutputID) {
			return u.propagateBranchUpdates(transactionID, conflictBranchID)
		}, types.True)
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}
}

// propagateBranchUpdates is an internal utility function that propagates changes in the perception of the BranchDAG
// after introducing a new ConflictBranch.
func (u *UTXODAG) propagateBranchUpdates(transactionID TransactionID, conflictBranchID BranchID) (updatedOutputs []OutputID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		if transactionMetadata.IsConflicting() {
			if err := u.ledgerstate.UpdateConflictBranchParents(transactionMetadata.BranchID(), u.consumedBranchIDs(transactionID)); err != nil {
				panic(fmt.Errorf("failed to update ConflictBranch with %s: %w", transactionMetadata.BranchID(), err))
			}
			return
		}

		updatedOutputs = u.updateBranchOfTransaction(transactionID, u.ledgerstate.AggregateConflictBranchesID(u.consumedBranchIDs(transactionID)), conflictBranchID)
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}

	return
}

// updateBranchOfTransaction is an internal utility function that updates the Branch that a Transaction and its Outputs
// are booked into.
func (u *UTXODAG) updateBranchOfTransaction(transactionID TransactionID, newBranchID, conflictBranchID BranchID) (updatedOutputs []OutputID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		if transactionMetadata.SetBranchID(newBranchID) {
			updatedOutputs = u.createdOutputIDsOfTransaction(transactionID)
			for _, outputID := range updatedOutputs {
				if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
					outputMetadata.SetBranchID(newBranchID)
				}) {
					panic(fmt.Errorf("failed to load OutputMetadata with %s", outputID))
				}
			}

			u.Events().TransactionBranchIDUpdatedByFork.Trigger(&TransactionBranchIDUpdatedByForkEvent{
				TransactionID:  transactionID,
				NewBranchID:    newBranchID,
				ForkedBranchID: conflictBranchID,
			})
		}
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata with %s", transactionID))
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
		if !(&CachedConsumer{CachedObject: u.consumerStorage.ComputeIfAbsent(newConsumer.ObjectStorageKey(), func(key []byte) objectstorage.StorableObject {
			newConsumer.Persist()
			newConsumer.SetModified()

			return newConsumer
		})}).Consume(func(consumer *Consumer) {
			consumer.SetValid(valid)
		}) {
			panic("failed to update valid flag of Consumer")
		}
	}
}

// bookOutputs creates the Outputs and their corresponding OutputsMetadata in the object storage.
func (u *UTXODAG) bookOutputs(transaction *Transaction, targetBranch BranchID) {
	for _, output := range transaction.Essence().Outputs() {
		// replace ColorMint color with unique color based on OutputID
		updatedOutput := output.UpdateMintingColor()

		// store Output
		u.outputStorage.Store(updatedOutput).Release()

		// store OutputMetadata
		metadata := NewOutputMetadata(updatedOutput.ID())
		metadata.SetBranchID(targetBranch)
		metadata.SetSolid(true)
		u.outputMetadataStorage.Store(metadata).Release()
	}
}

// determineBookingDetails is an internal utility function that determines the information that are required to fully
// book a newly arrived Transaction into the UTXODAG using the metadata of its referenced Inputs.
func (u *UTXODAG) determineBookingDetails(inputsMetadata OutputsMetadata) (inheritedBranchIDs BranchIDs, conflictingInputs OutputsMetadata, err error) {
	conflictingInputs = inputsMetadata.SpentOutputsMetadata()
	inheritedBranchIDs, err = u.ledgerstate.ResolvePendingConflictBranchIDs(inputsMetadata.BranchIDs())
	if err != nil {
		err = errors.Errorf("failed to resolve pending branches: %w", cerrors.ErrFatal)
		return
	}

	return
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region private utility functions ////////////////////////////////////////////////////////////////////////////////////

// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
func (u *UTXODAG) ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs) {
	cachedInputs = make(CachedOutputs, 0)
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
func (u *UTXODAG) transactionInputsMetadata(transaction *Transaction) (cachedInputsMetadata CachedOutputsMetadata) {
	cachedInputsMetadata = make(CachedOutputsMetadata, 0)
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

	seenTransactions := set.New()
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

// consumedBranchIDs is an internal utility function that determines the list of BranchIDs that were consumed by the
// Inputs of the given Transaction.
func (u *UTXODAG) consumedBranchIDs(transactionID TransactionID) (branchIDs BranchIDs) {
	branchIDs = make(BranchIDs)
	if !u.CachedTransaction(transactionID).Consume(func(transaction *Transaction) {
		for _, input := range transaction.Essence().Inputs() {
			if !u.CachedOutputMetadata(input.(*UTXOInput).ReferencedOutputID()).Consume(func(outputMetadata *OutputMetadata) {
				branchIDs[outputMetadata.BranchID()] = types.Void
			}) {
				panic(fmt.Errorf("failed to load OutputMetadata with %s", input.(*UTXOInput).ReferencedOutputID()))
			}
		}
	}) {
		panic(fmt.Errorf("failed to load Transaction with %s", transactionID))
	}

	branchIDs, err := u.ledgerstate.ResolvePendingConflictBranchIDs(branchIDs)
	if err != nil {
		panic(err)
	}

	return
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
	NewBranchID    BranchID
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
	address  Address
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

// AddressOutputMappingFromBytes unmarshals a AddressOutputMapping from a sequence of bytes.
func AddressOutputMappingFromBytes(bytes []byte) (addressOutputMapping *AddressOutputMapping, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	if addressOutputMapping, err = AddressOutputMappingFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse AddressOutputMapping from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// AddressOutputMappingFromMarshalUtil unmarshals an AddressOutputMapping using a MarshalUtil (for easier unmarshalling).
func AddressOutputMappingFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (addressOutputMapping *AddressOutputMapping, err error) {
	addressOutputMapping = &AddressOutputMapping{}
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

// AddressOutputMappingFromObjectStorage is a factory method that creates a new AddressOutputMapping instance from a
// storage key of the object storage. It is used by the object storage, to create new instances of this entity.
func AddressOutputMappingFromObjectStorage(key []byte, _ []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = AddressOutputMappingFromBytes(key); err != nil {
		err = errors.Errorf("failed to parse AddressOutputMapping from bytes: %w", err)
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

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (a *AddressOutputMapping) Update(objectstorage.StorableObject) {
	panic("updates disabled")
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

// String returns a human-readable version of the CachedAddressOutputMapping.
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

// String returns a human-readable version of the CachedAddressOutputMappings.
func (c CachedAddressOutputMappings) String() string {
	structBuilder := stringify.StructBuilder("CachedAddressOutputMappings")
	for i, cachedAddressOutputMapping := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedAddressOutputMapping))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the object storage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{OutputIDLength, TransactionIDLength}...)

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
		err = errors.Errorf("failed to parse Consumer from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConsumerFromMarshalUtil unmarshals a Consumer using a MarshalUtil (for easier unmarshalling).
func ConsumerFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (consumer *Consumer, err error) {
	consumer = &Consumer{}
	if consumer.consumedInput, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
		return
	}
	if consumer.transactionID, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
		return
	}
	if consumer.valid, err = types.TriBoolFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse valid flag (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return
}

// ConsumerFromObjectStorage is a factory method that creates a new Consumer instance from a storage key of the
// object storage. It is used by the object storage, to create new instances of this entity.
func ConsumerFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ConsumerFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		err = errors.Errorf("failed to parse Consumer from bytes: %w", err)
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

// String returns a human-readable version of the Consumer.
func (c *Consumer) String() (humanReadableConsumer string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("transactionID", c.transactionID),
	)
}

// Update is disabled and panics if it ever gets called - it is required to match the StorableObject interface.
func (c *Consumer) Update(objectstorage.StorableObject) {
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

// String returns a human-readable version of the CachedConsumer.
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
func (c CachedConsumers) Unwrap() (unwrappedConsumers []*Consumer) {
	unwrappedConsumers = make([]*Consumer, len(c))
	for i, cachedConsumer := range c {
		untypedObject := cachedConsumer.Get()
		if untypedObject == nil {
			continue
		}

		typedObject := untypedObject.(*Consumer)
		if typedObject == nil || typedObject.IsDeleted() {
			continue
		}

		unwrappedConsumers[i] = typedObject
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

// String returns a human-readable version of the CachedConsumers.
func (c CachedConsumers) String() string {
	structBuilder := stringify.StructBuilder("CachedConsumers")
	for i, cachedConsumer := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedConsumer))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
