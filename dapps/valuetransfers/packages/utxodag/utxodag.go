package utxodag

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

type UTXODAG struct {
	tangle        *tangle.Tangle
	branchManager *branchmanager.BranchManager

	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	attachmentStorage          *objectstorage.ObjectStorage
	outputStorage              *objectstorage.ObjectStorage
	consumerStorage            *objectstorage.ObjectStorage

	Events *Events

	workerPool async.WorkerPool
}

func New(badgerInstance *badger.DB, tangle *tangle.Tangle) (result *UTXODAG) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &UTXODAG{
		tangle:        tangle,
		branchManager: branchmanager.New(badgerInstance),

		transactionStorage:         osFactory.New(osTransaction, osTransactionFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		transactionMetadataStorage: osFactory.New(osTransactionMetadata, osTransactionMetadataFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		attachmentStorage:          osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		outputStorage:              osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		consumerStorage:            osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second), osLeakDetectionOption),

		Events: newEvents(),
	}

	tangle.Events.PayloadSolid.Attach(events.NewClosure(result.ProcessSolidPayload))

	return
}

func (utxoDAG *UTXODAG) BranchManager() *branchmanager.BranchManager {
	return utxoDAG.branchManager
}

func (utxoDAG *UTXODAG) ProcessSolidPayload(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
	utxoDAG.workerPool.Submit(func() { utxoDAG.storeTransactionWorker(cachedPayload, cachedMetadata) })
}

func (utxoDAG *UTXODAG) Transaction(transactionId transaction.Id) *transaction.CachedTransaction {
	return &transaction.CachedTransaction{CachedObject: utxoDAG.transactionStorage.Load(transactionId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (utxoDAG *UTXODAG) TransactionMetadata(transactionId transaction.Id) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.Load(transactionId.Bytes())}
}

func (utxoDAG *UTXODAG) GetTransactionOutput(outputId transaction.OutputId) *CachedOutput {
	return &CachedOutput{CachedObject: utxoDAG.outputStorage.Load(outputId.Bytes())}
}

// GetConsumers retrieves the approvers of a payload from the object storage.
func (utxoDAG *UTXODAG) GetConsumers(outputId transaction.OutputId) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	utxoDAG.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputId.Bytes())

	return consumers
}

// GetAttachments retrieves the att of a payload from the object storage.
func (utxoDAG *UTXODAG) GetAttachments(transactionId transaction.Id) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	utxoDAG.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionId.Bytes())

	return attachments
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (utxoDAG *UTXODAG) Shutdown() *UTXODAG {
	utxoDAG.workerPool.ShutdownGracefully()

	utxoDAG.transactionStorage.Shutdown()
	utxoDAG.transactionMetadataStorage.Shutdown()
	utxoDAG.outputStorage.Shutdown()
	utxoDAG.consumerStorage.Shutdown()

	return utxoDAG
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (utxoDAG *UTXODAG) Prune() (err error) {
	if err = utxoDAG.branchManager.Prune(); err != nil {
		return
	}

	for _, storage := range []*objectstorage.ObjectStorage{
		utxoDAG.transactionStorage,
		utxoDAG.transactionMetadataStorage,
		utxoDAG.outputStorage,
		utxoDAG.consumerStorage,
	} {
		if err = storage.Prune(); err != nil {
			return
		}
	}

	return
}

func (utxoDAG *UTXODAG) storeTransactionWorker(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *tangle.CachedPayloadMetadata) {
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()

	// abort if the parameters are empty
	solidPayload := cachedPayload.Unwrap()
	if solidPayload == nil || cachedPayloadMetadata.Unwrap() == nil {
		return
	}

	// store objects in database
	cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := utxoDAG.storeTransactionModels(solidPayload)

	// abort if the attachment was previously processed already (nil == was not stored)
	if cachedAttachment == nil {
		cachedTransaction.Release()
		cachedTransactionMetadata.Release()

		return
	}

	// trigger events for a new transaction
	if transactionIsNew {
		utxoDAG.Events.TransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
	}

	// check solidity of transaction and its corresponding attachment
	utxoDAG.solidifyTransactionWorker(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
}

func (utxoDAG *UTXODAG) storeTransactionModels(solidPayload *payload.Payload) (cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment, transactionIsNew bool) {
	cachedTransaction = &transaction.CachedTransaction{CachedObject: utxoDAG.transactionStorage.ComputeIfAbsent(solidPayload.Transaction().Id().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionIsNew = true

		result := solidPayload.Transaction()
		result.Persist()
		result.SetModified()

		return result
	})}

	if transactionIsNew {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.Store(NewTransactionMetadata(solidPayload.Transaction().Id()))}

		// store references to the consumed outputs
		solidPayload.Transaction().Inputs().ForEach(func(outputId transaction.OutputId) bool {
			utxoDAG.consumerStorage.Store(NewConsumer(outputId, solidPayload.Transaction().Id())).Release()

			return true
		})
	} else {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.Load(solidPayload.Transaction().Id().Bytes())}
	}

	// store a reference from the transaction to the payload that attached it or abort, if we have processed this attachment already
	attachment, stored := utxoDAG.attachmentStorage.StoreIfAbsent(NewAttachment(solidPayload.Transaction().Id(), solidPayload.Id()))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}

	return
}

func (utxoDAG *UTXODAG) solidifyTransactionWorker(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetdata *CachedTransactionMetadata, attachment *CachedAttachment) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedTransaction, cachedTransactionMetdata, attachment})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedTransaction, currentCachedTransactionMetadata, currentCachedAttachment := utxoDAG.popElementsFromSolidificationStack(solidificationStack)
			defer currentCachedTransaction.Release()
			defer currentCachedTransactionMetadata.Release()
			defer currentCachedAttachment.Release()

			// unwrap cached objects
			currentTransaction := currentCachedTransaction.Unwrap()
			currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
			currentAttachment := currentCachedAttachment.Unwrap()

			// abort if any of the retrieved models is nil or payload is not solid or it was set as solid already
			if currentTransaction == nil || currentTransactionMetadata == nil || currentAttachment == nil {
				return
			}

			// abort if the transaction is not solid or invalid
			if transactionSolid, err := utxoDAG.isTransactionSolid(currentTransaction, currentTransactionMetadata); !transactionSolid || err != nil {
				if err != nil {
					// TODO: TRIGGER INVALID TX + REMOVE TXS THAT APPROVE IT
					fmt.Println(err, currentTransaction)
				}

				return
			}

			transactionBecameNewlySolid := currentTransactionMetadata.SetSolid(true)
			if !transactionBecameNewlySolid {
				// TODO: book attachment

				return
			}

			// ... and schedule check of approvers
			utxoDAG.ForEachConsumers(currentTransaction, func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment) {
				solidificationStack.PushBack([3]interface{}{cachedTransaction, transactionMetadata, cachedAttachment})
			})

			// TODO: BOOK TRANSACTION
			utxoDAG.bookTransaction(cachedTransaction.Retain(), cachedTransactionMetdata.Retain())
		}()
	}
}

func (utxoDAG *UTXODAG) popElementsFromSolidificationStack(stack *list.List) (*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment) {
	currentSolidificationEntry := stack.Front()
	cachedTransaction := currentSolidificationEntry.Value.([3]interface{})[0].(*transaction.CachedTransaction)
	cachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[1].(*CachedTransactionMetadata)
	cachedAttachment := currentSolidificationEntry.Value.([3]interface{})[2].(*CachedAttachment)
	stack.Remove(currentSolidificationEntry)

	return cachedTransaction, cachedTransactionMetadata, cachedAttachment
}

func (utxoDAG *UTXODAG) isTransactionSolid(tx *transaction.Transaction, metadata *TransactionMetadata) (bool, error) {
	// abort if any of the models are nil or has been deleted
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return false, nil
	}

	// abort if we have previously determined the solidity status of the transaction already
	if metadata.Solid() {
		return true, nil
	}

	// get outputs that were referenced in the transaction inputs
	cachedInputs := utxoDAG.getCachedOutputsFromTransactionInputs(tx)
	defer cachedInputs.Release()

	// check the solidity of the inputs and retrieve the consumed balances
	inputsSolid, consumedBalances, err := utxoDAG.checkTransactionInputs(cachedInputs)

	// abort if an error occurred or the inputs are not solid, yet
	if !inputsSolid || err != nil {
		return false, err
	}

	if !utxoDAG.checkTransactionOutputs(consumedBalances, tx.Outputs()) {
		return false, fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.Id())
	}

	return true, nil
}

func (utxoDAG *UTXODAG) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result CachedOutputs) {
	result = make(CachedOutputs)
	tx.Inputs().ForEach(func(inputId transaction.OutputId) bool {
		result[inputId] = utxoDAG.GetTransactionOutput(inputId)

		return true
	})

	return
}

func (utxoDAG *UTXODAG) checkTransactionInputs(cachedInputs CachedOutputs) (inputsSolid bool, consumedBalances map[balance.Color]int64, err error) {
	inputsSolid = true
	consumedBalances = make(map[balance.Color]int64)

	for _, cachedInput := range cachedInputs {
		if !cachedInput.Exists() {
			inputsSolid = false

			continue
		}

		// should never be nil as we check Exists() before
		input := cachedInput.Unwrap()

		// update solid status
		inputsSolid = inputsSolid && input.Solid()

		// calculate the input balances
		for _, inputBalance := range input.Balances() {
			var newBalance int64
			if currentBalance, balanceExists := consumedBalances[inputBalance.Color()]; balanceExists {
				// check overflows in the numbers
				if inputBalance.Value() > math.MaxInt64-currentBalance {
					err = fmt.Errorf("buffer overflow in balances of inputs")

					return
				}

				newBalance = currentBalance + inputBalance.Value()
			} else {
				newBalance = inputBalance.Value()
			}
			consumedBalances[inputBalance.Color()] = newBalance
		}
	}

	return
}

// checkTransactionOutputs is a utility function that returns true, if the outputs are consuming all of the given inputs
// (the sum of all the balance changes is 0). It also accounts for the ability to "recolor" coins during the creating of
// outputs. If this function returns false, then the outputs that are defined in the transaction are invalid and the
// transaction should be removed from the ledger state.
func (utxoDAG *UTXODAG) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
	// create a variable to keep track of outputs that create a new color
	var newlyColoredCoins int64

	// iterate through outputs and check them one by one
	aborted := !outputs.ForEach(func(address address.Address, balances []*balance.Balance) bool {
		for _, outputBalance := range balances {
			// abort if the output creates a negative or empty output
			if outputBalance.Value() <= 0 {
				return false
			}

			// sidestep logic if we have a newly colored output (we check the supply later)
			if outputBalance.Color() == balance.ColorNew {
				// catch overflows
				if newlyColoredCoins > math.MaxInt64-outputBalance.Value() {
					return false
				}

				newlyColoredCoins += outputBalance.Value()

				continue
			}

			// check if the used color does not exist in our supply
			availableBalance, spentColorExists := inputBalances[outputBalance.Color()]
			if !spentColorExists {
				return false
			}

			// abort if we spend more coins of the given color than we have
			if availableBalance < outputBalance.Value() {
				return false
			}

			// subtract the spent coins from the supply of this transaction
			inputBalances[outputBalance.Color()] -= outputBalance.Value()

			// cleanup the entry in the supply map if we have exhausted all funds
			if inputBalances[outputBalance.Color()] == 0 {
				delete(inputBalances, outputBalance.Color())
			}
		}

		return true
	})

	// abort if the previous checks failed
	if aborted {
		return false
	}

	// determine the unspent inputs
	var unspentCoins int64
	for _, unspentBalance := range inputBalances {
		// catch overflows
		if unspentCoins > math.MaxInt64-unspentBalance {
			return false
		}

		unspentCoins += unspentBalance
	}

	// the outputs are valid if they spend all outputs
	return unspentCoins == newlyColoredCoins
}

func (utxoDAG *UTXODAG) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment)) {
	seenTransactions := make(map[transaction.Id]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		utxoDAG.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
			if _, transactionSeen := seenTransactions[consumer.TransactionId()]; !transactionSeen {
				seenTransactions[consumer.TransactionId()] = types.Void

				cachedTransaction := utxoDAG.Transaction(consumer.TransactionId())
				cachedTransactionMetadata := utxoDAG.TransactionMetadata(consumer.TransactionId())
				for _, cachedAttachment := range utxoDAG.GetAttachments(consumer.TransactionId()) {
					consume(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
				}
			}
		})

		return true
	})
}

func (utxoDAG *UTXODAG) bookTransaction(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) (err error) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	transactionToBook := cachedTransaction.Unwrap()
	if transactionToBook == nil {
		err = errors.New("failed to unwrap transaction")

		return
	}

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		err = errors.New("failed to unwrap transaction metadata")

		return
	}

	consumedBranches := make(branchmanager.BranchIds)
	conflictingInputs := make([]transaction.OutputId, 0)
	conflictingInputsOfConflictingConsumers := make(map[transaction.Id][]transaction.OutputId)

	if !transactionToBook.Inputs().ForEach(func(outputId transaction.OutputId) bool {
		cachedOutput := utxoDAG.GetTransactionOutput(outputId)
		defer cachedOutput.Release()

		// abort if the output could not be found
		output := cachedOutput.Unwrap()
		if output == nil {
			err = fmt.Errorf("could not load output '%s'", outputId)

			return false
		}

		consumedBranches[output.BranchId()] = types.Void

		// continue if we are the first consumer and there is no double spend
		consumerCount, firstConsumerId := output.RegisterConsumer(transactionToBook.Id())
		if consumerCount == 0 {
			return true
		}

		// keep track of conflicting inputs
		conflictingInputs = append(conflictingInputs, outputId)

		// also keep track of conflicting inputs of previous consumers
		if consumerCount == 1 {
			if _, conflictingInputsExist := conflictingInputsOfConflictingConsumers[firstConsumerId]; !conflictingInputsExist {
				conflictingInputsOfConflictingConsumers[firstConsumerId] = make([]transaction.OutputId, 0)
			}

			conflictingInputsOfConflictingConsumers[firstConsumerId] = append(conflictingInputsOfConflictingConsumers[firstConsumerId], outputId)
		}

		return true
	}) {
		return
	}

	cachedTargetBranch, _ := utxoDAG.branchManager.InheritBranches(consumedBranches.ToList()...)
	defer cachedTargetBranch.Release()

	targetBranch := cachedTargetBranch.Unwrap()
	if targetBranch == nil {
		return errors.New("failed to unwrap target branch")
	}
	targetBranch.Persist()

	if len(conflictingInputs) >= 1 {
		cachedTargetBranch = utxoDAG.branchManager.AddBranch(branchmanager.NewBranch(branchmanager.NewBranchId(transactionToBook.Id()), []branchmanager.BranchId{targetBranch.Id()}, conflictingInputs))
		defer cachedTargetBranch.Release()

		targetBranch = cachedTargetBranch.Unwrap()
		if targetBranch == nil {
			return errors.New("failed to inherit branches")
		}

		// TODO: CREATE / RETRIEVE CONFLICT SETS + ADD TARGET REALITY TO THEM
		/*
			for _, conflictingInput := range conflictingInputs {

			}
		*/
	}

	// book transaction into target reality
	transactionMetadata.SetBranchId(targetBranch.Id())

	// book outputs into the target branch
	transactionToBook.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		newOutput := NewOutput(address, transactionToBook.Id(), targetBranch.Id(), balances)
		newOutput.SetSolid(true)
		utxoDAG.outputStorage.Store(newOutput).Release()

		return true
	})

	// fork the conflicting transactions into their own branch
	previousConsumerForked := false
	for consumerId, conflictingInputs := range conflictingInputsOfConflictingConsumers {
		consumerForked, forkedErr := utxoDAG.Fork(consumerId, conflictingInputs)
		if forkedErr != nil {
			err = forkedErr

			return
		}

		previousConsumerForked = previousConsumerForked || consumerForked
	}

	// trigger events
	utxoDAG.Events.TransactionBooked.Trigger(cachedTransaction, cachedTransactionMetadata, cachedTargetBranch, conflictingInputs, previousConsumerForked)

	// TODO: BOOK ATTACHMENT

	return
}

func (utxoDAG *UTXODAG) calculateBranchOfTransaction(currentTransaction *transaction.Transaction) (branch *branchmanager.CachedBranch, err error) {
	consumedBranches := make(branchmanager.BranchIds)
	if !currentTransaction.Inputs().ForEach(func(outputId transaction.OutputId) bool {
		cachedTransactionOutput := utxoDAG.GetTransactionOutput(outputId)
		defer cachedTransactionOutput.Release()

		transactionOutput := cachedTransactionOutput.Unwrap()
		if transactionOutput == nil {
			err = fmt.Errorf("failed to load output '%s'", outputId)

			return false
		}

		consumedBranches[transactionOutput.BranchId()] = types.Void

		return true
	}) {
		return
	}

	branch, err = utxoDAG.branchManager.InheritBranches(consumedBranches.ToList()...)

	return
}

func (utxoDAG *UTXODAG) moveTransactionToBranch(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedTargetBranch *branchmanager.CachedBranch) (err error) {
	// push transaction that shall be moved to the stack
	transactionStack := list.New()
	branchStack := list.New()
	branchStack.PushBack([3]interface{}{cachedTransactionMetadata.Unwrap().BranchId(), cachedTargetBranch, transactionStack})
	transactionStack.PushBack([2]interface{}{cachedTransaction, cachedTransactionMetadata})

	// iterate through all transactions (grouped by their branch)
	for branchStack.Len() >= 1 {
		if err = func() error {
			// retrieve branch details from stack
			currentSolidificationEntry := branchStack.Front()
			currentSourceBranch := currentSolidificationEntry.Value.([3]interface{})[0].(branchmanager.BranchId)
			currentCachedTargetBranch := currentSolidificationEntry.Value.([3]interface{})[1].(*branchmanager.CachedBranch)
			transactionStack := currentSolidificationEntry.Value.([3]interface{})[2].(*list.List)
			branchStack.Remove(currentSolidificationEntry)
			defer currentCachedTargetBranch.Release()

			// unpack target branch
			targetBranch := currentCachedTargetBranch.Unwrap()
			if targetBranch == nil {
				return errors.New("failed to unpack branch")
			}

			// iterate through transactions
			for transactionStack.Len() >= 1 {
				if err = func() error {
					// retrieve transaction details from stack
					currentSolidificationEntry := transactionStack.Front()
					currentCachedTransaction := currentSolidificationEntry.Value.([2]interface{})[0].(*transaction.CachedTransaction)
					currentCachedTransactionMetadata := currentSolidificationEntry.Value.([2]interface{})[1].(*CachedTransactionMetadata)
					transactionStack.Remove(currentSolidificationEntry)
					defer currentCachedTransaction.Release()
					defer currentCachedTransactionMetadata.Release()

					// unwrap transaction
					currentTransaction := currentCachedTransaction.Unwrap()
					if currentTransaction == nil {
						return errors.New("failed to unwrap transaction")
					}

					// unwrap transaction metadata
					currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
					if currentTransactionMetadata == nil {
						return errors.New("failed to unwrap transaction metadata")
					}

					// if we arrived at a nested branch
					if currentTransactionMetadata.BranchId() != currentSourceBranch {
						// determine the new branch of the transaction
						newCachedTargetBranch, branchErr := utxoDAG.calculateBranchOfTransaction(currentTransaction)
						if branchErr != nil {
							return branchErr
						}
						defer newCachedTargetBranch.Release()

						// unwrap the branch
						newTargetBranch := newCachedTargetBranch.Unwrap()
						if newTargetBranch == nil {
							return errors.New("failed to unwrap branch")
						}
						newTargetBranch.Persist()

						// add the new branch (with the current transaction as a starting point to the branch stack)
						newTransactionStack := list.New()
						newTransactionStack.PushBack([2]interface{}{currentCachedTransaction.Retain(), currentCachedTransactionMetadata.Retain()})
						branchStack.PushBack([3]interface{}{currentTransactionMetadata.BranchId(), newCachedTargetBranch.Retain(), newTransactionStack})

						return nil
					}

					// abort if we did not modify the branch of the transaction
					if !currentTransactionMetadata.SetBranchId(targetBranch.Id()) {
						return nil
					}

					// iterate through the outputs of the moved transaction
					currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
						// create reference to the output
						outputId := transaction.NewOutputId(address, currentTransaction.Id())

						// load output from database
						cachedOutput := utxoDAG.GetTransactionOutput(outputId)
						defer cachedOutput.Release()

						// unwrap output
						output := cachedOutput.Unwrap()
						if output == nil {
							err = fmt.Errorf("failed to load output '%s'", outputId)

							return false
						}

						// abort if the output was moved already
						if !output.SetBranchId(targetBranch.Id()) {
							return true
						}

						// schedule consumers for further checks
						consumingTransactions := make(map[transaction.Id]types.Empty)
						utxoDAG.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
							consumingTransactions[consumer.TransactionId()] = types.Void
						})
						for transactionId := range consumingTransactions {
							transactionStack.PushBack([2]interface{}{utxoDAG.Transaction(transactionId), utxoDAG.TransactionMetadata(transactionId)})
						}

						return true
					})

					return nil
				}(); err != nil {
					return err
				}
			}

			return nil
		}(); err != nil {
			return
		}
	}

	return
}

func (utxoDAG *UTXODAG) Fork(transactionId transaction.Id, conflictingInputs []transaction.OutputId) (forked bool, err error) {
	cachedTransaction := utxoDAG.Transaction(transactionId)
	cachedTransactionMetadata := utxoDAG.TransactionMetadata(transactionId)
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	tx := cachedTransaction.Unwrap()
	if tx == nil {
		err = fmt.Errorf("failed to load transaction '%s'", transactionId)

		return
	}
	txMetadata := cachedTransactionMetadata.Unwrap()
	if txMetadata == nil {
		err = fmt.Errorf("failed to load metadata of transaction '%s'", transactionId)

		return
	}

	// abort if this transaction was finalized already
	if txMetadata.Finalized() {
		return
	}

	cachedTargetBranch := utxoDAG.branchManager.AddBranch(branchmanager.NewBranch(branchmanager.NewBranchId(tx.Id()), []branchmanager.BranchId{txMetadata.BranchId()}, conflictingInputs))
	defer cachedTargetBranch.Release()

	targetBranch := cachedTargetBranch.Unwrap()
	if targetBranch == nil {
		err = fmt.Errorf("failed to create branch for transaction '%s'", transactionId)

		return
	}

	if err = utxoDAG.moveTransactionToBranch(cachedTransaction.Retain(), cachedTransactionMetadata.Retain(), cachedTargetBranch.Retain()); err != nil {
		return
	}

	utxoDAG.Events.Fork.Trigger(cachedTransaction, cachedTransactionMetadata, targetBranch, conflictingInputs)
	forked = true

	return
}
