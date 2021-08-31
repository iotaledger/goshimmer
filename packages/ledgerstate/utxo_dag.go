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
	"github.com/iotaledger/hive.go/datastructure/walker"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/syncutils"
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
	// StoreTransaction adds a new Transaction to the ledger state. It returns a boolean that indicates whether the
	// Transaction was stored, its SolidityType and an error value that contains the cause for possibly exceptions.
	StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error)
	// CheckTransaction contains fast checks that have to be performed before booking a Transaction.
	CheckTransaction(transaction *Transaction) (err error)
	// CachedTransaction retrieves the Transaction with the given TransactionID from the object storage.
	CachedTransaction(transactionID TransactionID) (cachedTransaction *CachedTransaction)
	// Transaction returns a specific transaction, consumed.
	Transaction(transactionID TransactionID) (transaction *Transaction)
	// Transactions returns all the transactions, consumed.
	Transactions() (transactions map[TransactionID]*Transaction)
	// CachedTransactionMetadata retrieves the TransactionMetadata with the given TransactionID from the object storage.
	CachedTransactionMetadata(transactionID TransactionID, computeIfAbsentCallback ...func(transactionID TransactionID) *TransactionMetadata) (cachedTransactionMetadata *CachedTransactionMetadata)
	// CachedOutput retrieves the Output with the given OutputID from the object storage.
	CachedOutput(outputID OutputID) (cachedOutput *CachedOutput)
	// CachedOutputMetadata retrieves the OutputMetadata with the given OutputID from the object storage.
	CachedOutputMetadata(outputID OutputID) (cachedOutput *CachedOutputMetadata)
	// CachedConsumers retrieves the Consumers of the given OutputID from the object storage.
	CachedConsumers(outputID OutputID, optionalSolidityType ...SolidityType) (cachedConsumers CachedConsumers)
	// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
	LoadSnapshot(snapshot *Snapshot)
	// CachedAddressOutputMapping retrieves the outputs for the given address.
	CachedAddressOutputMapping(address Address) (cachedAddressOutputMappings CachedAddressOutputMappings)
	// ConsumedOutputs returns the consumed (cached)Outputs of the given Transaction.
	ConsumedOutputs(transaction *Transaction) (cachedInputs CachedOutputs)
	// ManageStoreAddressOutputMapping mangages how to store the address-output mapping dependent on which type of output it is.
	ManageStoreAddressOutputMapping(output Output)
	// StoreAddressOutputMapping stores the address-output mapping.
	StoreAddressOutputMapping(address Address, outputID OutputID)
}

// UTXODAG represents the DAG that is formed by Transactions consuming Inputs and creating Outputs. It forms the core of
// the ledger state and keeps track of the balances and the different perceptions of potential conflicts.
type UTXODAG struct {
	events *UTXODAGEvents

	transactionStorage          *objectstorage.ObjectStorage
	transactionMetadataStorage  *objectstorage.ObjectStorage
	outputStorage               *objectstorage.ObjectStorage
	outputMetadataStorage       *objectstorage.ObjectStorage
	consumerStorage             *objectstorage.ObjectStorage
	addressOutputMappingStorage *objectstorage.ObjectStorage
	branchDAG                   *BranchDAG
	shutdownOnce                sync.Once

	syncutils.MultiMutex
}

// NewUTXODAG create a new UTXODAG from the given details.
func NewUTXODAG(store kvstore.KVStore, cacheProvider *database.CacheTimeProvider, branchDAG *BranchDAG) (utxoDAG *UTXODAG) {
	options := buildObjectStorageOptions(cacheProvider)
	osFactory := objectstorage.NewFactory(store, database.PrefixLedgerState)
	utxoDAG = &UTXODAG{
		events: &UTXODAGEvents{
			TransactionBranchIDUpdated: events.NewEvent(TransactionIDEventHandler),
			TransactionSolid:           events.NewEvent(TransactionIDEventHandler),
		},
		transactionStorage:          osFactory.New(PrefixTransactionStorage, TransactionFromObjectStorage, options.transactionStorageOptions...),
		transactionMetadataStorage:  osFactory.New(PrefixTransactionMetadataStorage, TransactionMetadataFromObjectStorage, options.transactionMetadataStorageOptions...),
		outputStorage:               osFactory.New(PrefixOutputStorage, OutputFromObjectStorage, options.outputStorageOptions...),
		outputMetadataStorage:       osFactory.New(PrefixOutputMetadataStorage, OutputMetadataFromObjectStorage, options.outputMetadataStorageOptions...),
		consumerStorage:             osFactory.New(PrefixConsumerStorage, ConsumerFromObjectStorage, options.consumerStorageOptions...),
		addressOutputMappingStorage: osFactory.New(PrefixAddressOutputMappingStorage, AddressOutputMappingFromObjectStorage, options.addressOutputMappingStorageOptions...),
		branchDAG:                   branchDAG,
	}
	return
}

// Events returns all events of the UTXODAG
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

// StoreTransaction adds a new Transaction to the ledger state. It returns a boolean that indicates whether the
// Transaction was stored, its SolidityType and an error value that contains the cause for possibly exceptions.
func (u *UTXODAG) StoreTransaction(transaction *Transaction) (stored bool, solidityType SolidityType, err error) {
	propagationWalker := walker.New(true)

	eventQueue := eventsNewQueue()

	u.LockEntity(transaction)
	u.CachedTransactionMetadata(transaction.ID(), func(transactionID TransactionID) *TransactionMetadata {
		u.transactionStorage.Store(transaction).Release()
		transactionMetadata := NewTransactionMetadata(transactionID)

		err = u.solidifyTransaction(transaction, transactionMetadata, eventQueue, propagationWalker)
		stored = true

		return transactionMetadata
	}).Consume(func(transactionMetadata *TransactionMetadata) {
		solidityType = transactionMetadata.SolidityType()
	})
	u.UnlockEntity(transaction)

	eventQueue.Trigger()

	for propagationWalker.HasNext() {
		transactionID := propagationWalker.Next().(TransactionID)

		u.CachedTransaction(transactionID).Consume(func(transaction *Transaction) {
			defer eventQueue.Trigger()

			u.LockEntity(transaction)
			defer u.UnlockEntity(transaction)

			u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
				_ = u.solidifyTransaction(transaction, transactionMetadata, eventQueue, propagationWalker)
			})
		})
	}

	return stored, solidityType, err
}

// CheckTransaction checks if a Transaction is objectively valid.
func (u *UTXODAG) CheckTransaction(transaction *Transaction) (err error) {
	cachedConsumedOutputs := u.ConsumedOutputs(transaction)
	defer cachedConsumedOutputs.Release()
	consumedOutputs := cachedConsumedOutputs.Unwrap()

	// perform cheap checks
	if !u.allOutputsExist(consumedOutputs) {
		return errors.Errorf("not all consumedOutputs of transaction are solid: %w", ErrTransactionNotSolid)
	}

	return u.transactionObjectivelyValid(transaction, consumedOutputs)
}

// InclusionState returns the InclusionState of the Transaction with the given TransactionID which can either be
// Pending, Confirmed or Rejected.
func (u *UTXODAG) GradeOfFinality(transactionID TransactionID) (gradeOfFinality gof.GradeOfFinality, err error) {
	cachedTransactionMetadata := u.CachedTransactionMetadata(transactionID)
	defer cachedTransactionMetadata.Release()
	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		err = errors.Errorf("failed to load TransactionMetadata with %s: %w", transactionID, cerrors.ErrFatal)
		return
	}

	cachedBranch := u.branchDAG.Branch(transactionMetadata.BranchID())
	defer cachedBranch.Release()
	branch := cachedBranch.Unwrap()
	if branch == nil {
		err = errors.Errorf("failed to load Branch with %s: %w", transactionMetadata.BranchID(), cerrors.ErrFatal)
		return
	}

	gradeOfFinality = branch.GradeOfFinality()

	return
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
func (u *UTXODAG) CachedTransactionMetadata(transactionID TransactionID, computeIfAbsentCallback ...func(transactionID TransactionID) *TransactionMetadata) (cachedTransactionMetadata *CachedTransactionMetadata) {
	if len(computeIfAbsentCallback) >= 1 {
		return &CachedTransactionMetadata{u.transactionMetadataStorage.ComputeIfAbsent(transactionID.Bytes(), func(key []byte) objectstorage.StorableObject {
			return computeIfAbsentCallback[0](transactionID)
		})}
	}

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
func (u *UTXODAG) CachedConsumers(outputID OutputID, optionalSolidityType ...SolidityType) (cachedConsumers CachedConsumers) {
	var iterationPrefix []byte
	if len(optionalSolidityType) >= 1 {
		iterationPrefix = byteutils.ConcatBytes(outputID.Bytes(), optionalSolidityType[0].Bytes())
	} else {
		iterationPrefix = outputID.Bytes()
	}

	cachedConsumers = make(CachedConsumers, 0)
	u.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		cachedConsumers = append(cachedConsumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, objectstorage.WithIteratorPrefix(iterationPrefix))

	return
}

// LoadSnapshot creates a set of outputs in the UTXO-DAG, that are forming the genesis for future transactions.
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
			metadata.SetGradeOfFinality(gof.High)
			cachedMetadata, stored := u.outputMetadataStorage.StoreIfAbsent(metadata)
			if stored {
				cachedMetadata.Release()
			}
		}

		// store TransactionMetadata
		txMetadata := NewTransactionMetadata(txID)
		txMetadata.SetSolidityType(Solid)
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

func (u *UTXODAG) solidifyTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, eventsQueue *eventsQueue, propagationWalker *walker.Walker) (err error) {
	cachedConsumedOutputs := u.ConsumedOutputs(transaction)
	defer cachedConsumedOutputs.Release()
	consumedOutputs := cachedConsumedOutputs.Unwrap()

	if !u.allOutputsExist(consumedOutputs) {
		previousSolidityType := transactionMetadata.SetSolidityType(Unsolid)
		if previousSolidityType >= Unsolid {
			return
		}

		u.updateConsumers(transaction, previousSolidityType, Unsolid)

		return
	}

	if validErr := u.transactionObjectivelyValid(transaction, consumedOutputs); validErr != nil {
		eventsQueue.Queue(u.Events().TransactionInvalid, transaction, validErr)

		return
	}

	if _, err = u.bookTransaction(transaction, transactionMetadata, consumedOutputs); err != nil {
		err = errors.Errorf("failed to book Transaction with %s: %w", transaction.ID(), err)

		eventsQueue.Queue(u.Events().Error, transaction, err)

		return
	}

	for _, output := range transaction.Essence().Outputs() {
		u.ManageStoreAddressOutputMapping(output)
	}

	eventsQueue.Queue(u.Events().TransactionSolid, transaction.ID())

	for transactionID := range u.consumingTransactionIDs(transaction, Unsolid) {
		propagationWalker.Push(transactionID)
	}

	return err
}

func (u *UTXODAG) updateConsumers(transaction *Transaction, previousSolidityType, newSolidityType SolidityType) {
	if previousSolidityType == newSolidityType {
		return
	}

	for _, input := range transaction.Essence().Inputs() {
		if previousSolidityType != UndefinedSolidityType {
			u.consumerStorage.Delete(NewConsumer(input.(*UTXOInput).ReferencedOutputID(), transaction.ID(), previousSolidityType).Bytes())
		}

		u.consumerStorage.Store(NewConsumer(input.(*UTXOInput).ReferencedOutputID(), transaction.ID(), newSolidityType)).Release()
	}
}

func (u *UTXODAG) transactionObjectivelyValid(transaction *Transaction, consumedOutputs Outputs) (err error) {
	if !TransactionBalancesValid(consumedOutputs, transaction.Essence().Outputs()) {
		return errors.Errorf("sum of consumed and spent balances is not 0: %w", ErrTransactionInvalid)
	}

	if !UnlockBlocksValid(consumedOutputs, transaction) {
		return errors.Errorf("spending of referenced consumedOutputs is not authorized: %w", ErrTransactionInvalid)
	}

	if !AliasInitialStateValid(consumedOutputs, transaction) {
		return errors.Errorf("initial state of created alias output is invalid: %w", ErrTransactionInvalid)
	}

	return nil
}

func (u *UTXODAG) bookTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, consumedOutputs Outputs) (targetBranch BranchID, err error) {
	// retrieve the metadata of the Inputs
	cachedInputsMetadata := u.transactionInputsMetadata(transaction)
	defer cachedInputsMetadata.Release()
	inputsMetadata := cachedInputsMetadata.Unwrap()

	// check if Transaction is attaching to something invalid
	if u.inputsInInvalidBranch(inputsMetadata) {
		u.bookInvalidTransaction(transaction, transactionMetadata)

		return InvalidBranchID, nil
	}

	// check if transaction is attaching to something rejected
	if rejected, rejectedBranch := u.inputsInRejectedBranch(inputsMetadata); rejected {
		u.bookRejectedTransaction(transaction, transactionMetadata, rejectedBranch)

		return rejectedBranch, nil
	}

	// check if any Input was spent by a confirmed Transaction already
	if inputsSpentByConfirmedTransaction, tmpErr := u.inputsSpentByConfirmedTransaction(inputsMetadata); tmpErr != nil {
		return BranchID{}, errors.Errorf("failed to check if inputs were spent by confirmed Transaction: %w", err)
	} else if inputsSpentByConfirmedTransaction {
		return u.bookRejectedConflictingTransaction(transaction, transactionMetadata)
	}

	// mark transaction as "permanently rejected"
	if !u.consumedOutputsPastConeValid(consumedOutputs, inputsMetadata) {
		u.bookInvalidTransaction(transaction, transactionMetadata)

		return InvalidBranchID, nil
	}

	// determine the booking details before we book
	branchesOfInputsConflicting, normalizedBranchIDs, conflictingInputs, err := u.determineBookingDetails(inputsMetadata)
	if err != nil {
		return BranchID{}, errors.Errorf("failed to determine booking details of Transaction with %s: %w", transaction.ID(), err)
	}

	// are branches of inputs conflicting
	if branchesOfInputsConflicting {
		u.bookInvalidTransaction(transaction, transactionMetadata)

		return InvalidBranchID, nil
	}

	if len(conflictingInputs) != 0 {
		return u.bookConflictingTransaction(transaction, transactionMetadata, inputsMetadata, normalizedBranchIDs, conflictingInputs.ByID()), nil
	}

	return u.bookNonConflictingTransaction(transaction, transactionMetadata, inputsMetadata, normalizedBranchIDs), nil
}

func (u *UTXODAG) consumingTransactionIDs(transaction *Transaction, optionalSolidityType ...SolidityType) (consumingTransactionIDs TransactionIDs) {
	consumingTransactionIDs = make(TransactionIDs)
	for _, output := range transaction.Essence().Outputs() {
		u.CachedConsumers(output.ID(), optionalSolidityType...).Consume(func(consumer *Consumer) {
			consumingTransactionIDs[consumer.TransactionID()] = types.Void
		})
	}

	return consumingTransactionIDs
}

// bookInvalidTransaction is an internal utility function that books the given Transaction into the Branch identified by
// the InvalidBranchID.
func (u *UTXODAG) bookInvalidTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata) {
	transactionMetadata.SetBranchID(InvalidBranchID)
	u.updateConsumers(transaction, transactionMetadata.SetSolidityType(Invalid), Invalid)
	transactionMetadata.SetGradeOfFinality(gof.High)

	u.bookOutputs(transaction, InvalidBranchID)
}

// bookRejectedTransaction is an internal utility function that "lazy" books the given Transaction into a rejected
// Branch.
func (u *UTXODAG) bookRejectedTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, rejectedBranch BranchID) {
	transactionMetadata.SetBranchID(rejectedBranch)
	u.updateConsumers(transaction, transactionMetadata.SetSolidityType(LazySolid), LazySolid)

	u.bookOutputs(transaction, rejectedBranch)
}

// bookRejectedConflictingTransaction is an internal utility function that "lazy" books the given Transaction into its
// own ConflictBranch which is immediately rejected and only kept in the DAG for possible reorgs.
func (u *UTXODAG) bookRejectedConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata) (targetBranch BranchID, err error) {
	targetBranch = NewBranchID(transaction.ID())
	cachedConflictBranch, _, conflictBranchErr := u.branchDAG.CreateConflictBranch(targetBranch, NewBranchIDs(LazyBookedConflictsBranchID), nil)
	if conflictBranchErr != nil {
		err = errors.Errorf("failed to create ConflictBranch for lazy booked Transaction with %s: %w", transaction.ID(), conflictBranchErr)
		return
	}
	if !cachedConflictBranch.Consume(func(branch Branch) {
		u.bookRejectedTransaction(transaction, transactionMetadata, targetBranch)
	}) {
		err = errors.Errorf("failed to load ConflictBranch with %s: %w", cachedConflictBranch.ID(), cerrors.ErrFatal)
	}
	return
}

// bookNonConflictingTransaction is an internal utility function that books the Transaction into the Branch that is
// determined by aggregating the Branches of the consumed Inputs.
func (u *UTXODAG) bookNonConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, normalizedBranchIDs BranchIDs) (targetBranch BranchID) {
	cachedAggregatedBranch, _, err := u.branchDAG.aggregateNormalizedBranches(normalizedBranchIDs)
	if err != nil {
		panic(fmt.Errorf("failed to aggregate Branches when booking Transaction with %s: %w", transaction.ID(), err))
	}

	if !cachedAggregatedBranch.Consume(func(branch Branch) {
		targetBranch = branch.ID()

		transactionMetadata.SetBranchID(targetBranch)
		if previousSolidityType := transactionMetadata.SetSolidityType(Solid); previousSolidityType != Solid {
			u.updateConsumers(transaction, previousSolidityType, Solid)

			for _, inputMetadata := range inputsMetadata {
				inputMetadata.RegisterConsumer(transaction.ID())
			}
		}
		u.bookOutputs(transaction, targetBranch)
	}) {
		panic(fmt.Errorf("failed to load AggregatedBranch with %s", cachedAggregatedBranch.ID()))
	}

	return
}

// bookConflictingTransaction is an internal utility function that books a Transaction that uses Inputs that have
// already been spent by another Transaction. It create a new ConflictBranch for the new Transaction and "forks" the
// existing consumers of the conflicting Inputs.
func (u *UTXODAG) bookConflictingTransaction(transaction *Transaction, transactionMetadata *TransactionMetadata, inputsMetadata OutputsMetadata, normalizedBranchIDs BranchIDs, conflictingInputs OutputsMetadataByID) (targetBranch BranchID) {
	// fork existing consumers
	u.walkFutureCone(conflictingInputs.IDs(), func(transactionID TransactionID) (nextOutputsToVisit []OutputID) {
		u.forkConsumer(transactionID, conflictingInputs)

		return
	}, Solid)

	// create new ConflictBranch
	targetBranch = NewBranchID(transaction.ID())
	cachedConflictBranch, _, err := u.branchDAG.CreateConflictBranch(targetBranch, normalizedBranchIDs, conflictingInputs.ConflictIDs())
	if err != nil {
		panic(fmt.Errorf("failed to create ConflictBranch when booking Transaction with %s: %w", transaction.ID(), err))
	}

	// book Transaction into new ConflictBranch
	if !cachedConflictBranch.Consume(func(branch Branch) {
		transactionMetadata.SetBranchID(targetBranch)
		if previousSolidityType := transactionMetadata.SetSolidityType(Solid); previousSolidityType != Solid {
			u.updateConsumers(transaction, previousSolidityType, Solid)

			for _, inputMetadata := range inputsMetadata {
				inputMetadata.RegisterConsumer(transaction.ID())
			}
		}
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

		cachedConsumingConflictBranch, _, err := u.branchDAG.CreateConflictBranch(conflictBranchID, conflictBranchParents, conflictIDs)
		if err != nil {
			panic(fmt.Errorf("failed to create ConflictBranch when forking Transaction with %s: %w", transactionID, err))
		}
		// We don't need to propagate updates if the branch did already exist.
		// Though CreateConflictBranch needs to be called so that conflict sets and conflict membership are properly updated.
		if txMetadata.BranchID() == conflictBranchID {
			cachedConsumingConflictBranch.Release()
			return
		}

		cachedConsumingConflictBranch.Consume(func(newBranch Branch) {
			// copying the branch metadata properties from the original branch to the newly created.
			u.branchDAG.Branch(txMetadata.BranchID()).Consume(func(oldBranch Branch) {
				newBranch.SetGradeOfFinality(oldBranch.GradeOfFinality())
			})
		})

		txMetadata.SetBranchID(conflictBranchID)
		u.Events().TransactionBranchIDUpdated.Trigger(transactionID)

		outputIds := u.createdOutputIDsOfTransaction(transactionID)
		for _, outputID := range outputIds {
			if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
				outputMetadata.SetBranchID(conflictBranchID)
			}) {
				panic("failed to load OutputMetadata")
			}
		}

		u.walkFutureCone(outputIds, u.propagateBranchUpdates, Solid)
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}
}

// propagateBranchUpdates is an internal utility function that propagates changes in the perception of the BranchDAG
// after introducing a new ConflictBranch.
func (u *UTXODAG) propagateBranchUpdates(transactionID TransactionID) (updatedOutputs []OutputID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		// if the BranchID is the TransactionID we have a ConflictBranch
		if transactionMetadata.BranchID() == NewBranchID(transactionID) {
			if err := u.branchDAG.UpdateConflictBranchParents(transactionMetadata.BranchID(), u.consumedBranchIDs(transactionID)); err != nil {
				panic(fmt.Errorf("failed to update ConflictBranch with %s: %w", transactionMetadata.BranchID(), err))
			}
			return
		}

		cachedAggregatedBranch, _, err := u.branchDAG.AggregateBranches(u.consumedBranchIDs(transactionID))
		if err != nil {
			panic(err)
		}
		defer cachedAggregatedBranch.Release()

		updatedOutputs = u.updateBranchOfTransaction(transactionID, cachedAggregatedBranch.ID())
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata of Transaction with %s", transactionID))
	}

	return
}

// updateBranchOfTransaction is an internal utility function that updates the Branch that a Transaction and its Outputs
// are booked into.
func (u *UTXODAG) updateBranchOfTransaction(transactionID TransactionID, branchID BranchID) (updatedOutputs []OutputID) {
	if !u.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *TransactionMetadata) {
		if transactionMetadata.SetBranchID(branchID) {
			u.Events().TransactionBranchIDUpdated.Trigger(transactionID)

			updatedOutputs = u.createdOutputIDsOfTransaction(transactionID)
			for _, outputID := range updatedOutputs {
				if !u.CachedOutputMetadata(outputID).Consume(func(outputMetadata *OutputMetadata) {
					outputMetadata.SetBranchID(branchID)
				}) {
					panic(fmt.Errorf("failed to load OutputMetadata with %s", outputID))
				}
			}
		}
	}) {
		panic(fmt.Errorf("failed to load TransactionMetadata with %s", transactionID))
	}

	return
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
func (u *UTXODAG) determineBookingDetails(inputsMetadata OutputsMetadata) (branchesOfInputsConflicting bool, normalizedBranchIDs BranchIDs, conflictingInputs OutputsMetadata, err error) {
	conflictingInputs = make(OutputsMetadata, 0)
	consumedBranches := make([]BranchID, len(inputsMetadata))
	for i, inputMetadata := range inputsMetadata {
		consumedBranches[i] = inputMetadata.BranchID()

		if inputMetadata.ConsumerCount() >= 1 {
			conflictingInputs = append(conflictingInputs, inputMetadata)
		}
	}

	normalizedBranchIDs, err = u.branchDAG.normalizeBranches(NewBranchIDs(consumedBranches...))
	if err != nil {
		if errors.Is(err, ErrInvalidStateTransition) {
			branchesOfInputsConflicting = true
			err = nil
			return
		}

		err = errors.Errorf("failed to normalize branches: %w", cerrors.ErrFatal)
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

// allOutputsExist is an internal utility function that checks if all of the given Inputs exist.
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

// inputsInInvalidBranch is an internal utility function that checks if any of the Inputs is booked into the InvalidBranch.
func (u *UTXODAG) inputsInInvalidBranch(inputsMetadata OutputsMetadata) (invalid bool) {
	for _, inputMetadata := range inputsMetadata {
		if invalid = inputMetadata.BranchID() == InvalidBranchID; invalid {
			return
		}
	}

	return
}

// inputsInRejectedBranch checks if any of the Inputs is booked into a rejected Branch.
func (u *UTXODAG) inputsInRejectedBranch(inputsMetadata OutputsMetadata) (rejected bool, rejectedBranch BranchID) {
	seenBranchIDs := set.New()
	for _, inputMetadata := range inputsMetadata {
		if rejectedBranch = inputMetadata.BranchID(); !seenBranchIDs.Add(rejectedBranch) {
			continue
		}

		u.branchDAG.ForEachConflictingBranchID(rejectedBranch, func(conflictingBranchID BranchID) {
			u.branchDAG.Branch(conflictingBranchID).Consume(func(branch Branch) {
				rejected = rejected || branch.GradeOfFinality() == gof.High
			})
		})
		if rejected {
			return
		}
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

// inputsSpentByConfirmedTransaction is an internal utility function that checks if any of the given inputs was spent by
// a confirmed Transaction already.
func (u *UTXODAG) inputsSpentByConfirmedTransaction(inputsMetadata OutputsMetadata) (inputsSpentByConfirmedTransaction bool, err error) {
	for _, inputMetadata := range inputsMetadata {
		if inputMetadata.ConsumerCount() >= 1 {
			cachedConsumers := u.CachedConsumers(inputMetadata.ID())
			consumers := cachedConsumers.Unwrap()
			for _, consumer := range consumers {
				inclusionState, inclusionStateErr := u.GradeOfFinality(consumer.TransactionID())
				if inclusionStateErr != nil {
					cachedConsumers.Release()
					err = errors.Errorf("failed to determine InclusionState of Transaction with %s: %w", consumer.TransactionID(), inclusionStateErr)
					return
				}
				if inclusionState == gof.High {
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
func (u *UTXODAG) walkFutureCone(entryPoints []OutputID, callback func(transactionID TransactionID) (nextOutputsToVisit []OutputID), optionalValidFlagFilter ...SolidityType) {
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

			if len(optionalValidFlagFilter) >= 1 && consumer.SolidityType() != optionalValidFlagFilter[0] {
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

	return
}

// ManageStoreAddressOutputMapping mangages how to store the address-output mapping dependent on which type of output it is.
func (u *UTXODAG) ManageStoreAddressOutputMapping(output Output) {
	switch output.Type() {
	case AliasOutputType:
		castedOutput := output.(*AliasOutput)
		// if it is an origin alias output, we don't have the aliasaddress from the parsed bytes.
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

// TODO: IMPLEMENT A GOOD SYNCHRONIZATION MECHANISM FOR THE UTXODAG
/*
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
*/

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region UTXODAGEvents ////////////////////////////////////////////////////////////////////////////////////////////////

// UTXODAGEvents is a container for all of the UTXODAG related events.
type UTXODAGEvents struct {
	// Error is triggered when an unexpected error occurred in the component.
	Error *events.Event

	// TransactionInvalid gets triggered whenever an objectively invalid Transaction is detected.
	TransactionInvalid *events.Event

	// TransactionSolid gets triggered whenever a Transaction becomes LazySolid, Solid, or Invalid.
	TransactionSolid *events.Event

	// TransactionBranchIDUpdated gets triggered when the BranchID of a Transaction is changed after the initial booking.
	TransactionBranchIDUpdated *events.Event
}

// TransactionIDEventHandler is an event handler for an event with a TransactionID.
func TransactionIDEventHandler(handler interface{}, params ...interface{}) {
	handler.(func(TransactionID))(params[0].(TransactionID))
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

// AddressOutputMappingFromMarshalUtil unmarshals an AddressOutputMapping using a MarshalUtil (for easier unmarshaling).
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

// String returns a human readable version of the Consumer.
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

// region SolidityType /////////////////////////////////////////////////////////////////////////////////////////////////

// SolidityType is a type that specifies the types of solid states that a transaction can be in.
type SolidityType uint8

// SolidityTypeLength defines the amount of bytes of a marshaled SolidityType.
const SolidityTypeLength = 1

const (
	// UndefinedSolidityType represents the zero value of the SolidityType.
	UndefinedSolidityType SolidityType = iota

	// Unsolid represents transactions that are not solid, yet.
	Unsolid

	// LazySolid represents transactions that are solid but not fully booked because they spend funds of rejected or
	// invalid Branches.
	LazySolid

	// Solid represents transaction that are solid and fully booked.
	Solid

	// Invalid represents transactions that are solid and fully booked but spend outputs from conflicting branches.
	Invalid
)

// SolidityTypeFromBytes unmarshals a SolidityType from a sequence of bytes.
func SolidityTypeFromBytes(solidityTypeBytes []byte) (solidityType SolidityType, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(solidityTypeBytes)
	if solidityType, err = SolidityTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SolidityType from MarshalUtil: %w", err)
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// SolidityTypeFromMarshalUtil unmarshals a SolidityType using a MarshalUtil (for easier unmarshaling).
func SolidityTypeFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (solidityType SolidityType, err error) {
	untypedSolidityType, err := marshalUtil.ReadUint8()
	if err != nil {
		err = errors.Errorf("failed to parse SolidityType (%v): %w", err, cerrors.ErrParseBytesFailed)
		return
	}

	return SolidityType(untypedSolidityType), nil
}

// Bytes returns a marshaled version of the SolidityType.
func (c SolidityType) Bytes() (marshaledSolidityType []byte) {
	return []byte{uint8(c)}
}

// String returns a human readable version of the SolidityType.
func (c SolidityType) String() (humanReadableSolidityType string) {
	switch c {
	case UndefinedSolidityType:
		return "SolidityType(UndefinedSolidityType)"
	case Unsolid:
		return "SolidityType(Unsolid)"
	case Solid:
		return "SolidityType(Solid)"
	case LazySolid:
		return "SolidityType(LazySolid)"
	default:
		return "SolidityType(" + strconv.Itoa(int(c)) + ")"
	}
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

// region Consumer /////////////////////////////////////////////////////////////////////////////////////////////////////

// ConsumerPartitionKeys defines the "layout" of the key. This enables prefix iterations in the objectstorage.
var ConsumerPartitionKeys = objectstorage.PartitionKey([]int{OutputIDLength, SolidityTypeLength, TransactionIDLength}...)

// Consumer represents the relationship between an Output and its spending Transactions. Since an Output can have a
// potentially unbounded amount of spending Transactions, we store this as a separate k/v pair instead of a marshaled
// list of spending Transactions inside the Output.
type Consumer struct {
	solidityType  SolidityType
	consumedInput OutputID
	transactionID TransactionID

	objectstorage.StorableObjectFlags
}

// NewConsumer creates a Consumer object from the given information.
func NewConsumer(consumedInput OutputID, transactionID TransactionID, solidityType SolidityType) *Consumer {
	return &Consumer{
		consumedInput: consumedInput,
		transactionID: transactionID,
		solidityType:  solidityType,
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

// ConsumerFromMarshalUtil unmarshals an Consumer using a MarshalUtil (for easier unmarshaling).
func ConsumerFromMarshalUtil(marshalUtil *marshalutil.MarshalUtil) (consumer *Consumer, err error) {
	consumer = &Consumer{}
	if consumer.consumedInput, err = OutputIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse consumed Input from MarshalUtil: %w", err)
		return
	}
	if consumer.solidityType, err = SolidityTypeFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse SolidityType from MarshalUtil: %w", err)
		return
	}
	if consumer.transactionID, err = TransactionIDFromMarshalUtil(marshalUtil); err != nil {
		err = errors.Errorf("failed to parse TransactionID from MarshalUtil: %w", err)
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

// SolidityType returns the type of the Consumer.
func (c *Consumer) SolidityType() (solidityType SolidityType) {
	return c.solidityType
}

// TransactionID returns the TransactionID of the consuming Transaction.
func (c *Consumer) TransactionID() TransactionID {
	return c.transactionID
}

// Bytes marshals the Consumer into a sequence of bytes.
func (c *Consumer) Bytes() []byte {
	return byteutils.ConcatBytes(c.ObjectStorageKey(), c.ObjectStorageValue())
}

// String returns a human readable version of the Consumer.
func (c *Consumer) String() (humanReadableConsumer string) {
	return stringify.Struct("Consumer",
		stringify.StructField("consumedInput", c.consumedInput),
		stringify.StructField("solidityType", c.solidityType),
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
	return byteutils.ConcatBytes(c.consumedInput.Bytes(), c.solidityType.Bytes(), c.transactionID.Bytes())
}

// ObjectStorageValue marshals the Consumer into a sequence of bytes that are used as the value part in the object
// storage.
func (c *Consumer) ObjectStorageValue() []byte {
	return nil
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

// String returns a human readable version of the CachedConsumers.
func (c CachedConsumers) String() string {
	structBuilder := stringify.StructBuilder("CachedConsumers")
	for i, cachedConsumer := range c {
		structBuilder.AddField(stringify.StructField(strconv.Itoa(i), cachedConsumer))
	}

	return structBuilder.String()
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////
