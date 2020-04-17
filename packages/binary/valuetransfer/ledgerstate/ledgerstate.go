package ledgerstate

import (
	"container/list"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

type LedgerState struct {
	attachmentStorage *objectstorage.ObjectStorage

	tangle *tangle.Tangle

	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	outputStorage              *objectstorage.ObjectStorage
	consumerStorage            *objectstorage.ObjectStorage
	branchStorage              *objectstorage.ObjectStorage

	Events *Events

	storeTransactionWorkerPool async.WorkerPool
	solidifierWorkerPool       async.WorkerPool
}

func New(badgerInstance *badger.DB, tangle *tangle.Tangle) (result *LedgerState) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.LedgerState)

	leakDetectionOption := objectstorage.LeakDetectionEnabled(false, objectstorage.LeakDetectionOptions{
		MaxConsumersPerObject: 10,
		MaxConsumerHoldTime:   10 * time.Second,
	})

	result = &LedgerState{
		tangle:                     tangle,
		transactionStorage:         osFactory.New(osTransaction, osTransactionFactory, objectstorage.CacheTime(time.Second), leakDetectionOption),
		transactionMetadataStorage: osFactory.New(osTransactionMetadata, osTransactionMetadataFactory, objectstorage.CacheTime(time.Second), leakDetectionOption),
		attachmentStorage:          osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second), leakDetectionOption),
		outputStorage:              osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second), leakDetectionOption),
		consumerStorage:            osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second), leakDetectionOption),
		Events:                     newEvents(),
	}

	tangle.Events.PayloadSolid.Attach(events.NewClosure(result.ProcessSolidPayload))

	return
}

func (ledgerState *LedgerState) ProcessSolidPayload(cachedPayload *payload.CachedPayload, cachedMetadata *tangle.CachedPayloadMetadata) {
	ledgerState.storeTransactionWorkerPool.Submit(func() { ledgerState.storeTransactionWorker(cachedPayload, cachedMetadata) })
}

func (ledgerState *LedgerState) GetTransaction(transactionId transaction.Id) *transaction.CachedTransaction {
	return &transaction.CachedTransaction{CachedObject: ledgerState.transactionStorage.Load(transactionId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (ledgerState *LedgerState) GetTransactionMetadata(transactionId transaction.Id) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: ledgerState.transactionMetadataStorage.Load(transactionId.Bytes())}
}

func (ledgerState *LedgerState) GetTransactionOutput(outputId transaction.OutputId) *CachedOutput {
	return &CachedOutput{CachedObject: ledgerState.outputStorage.Load(outputId.Bytes())}
}

func (ledgerState *LedgerState) GetBranch(branchId BranchId) *CachedBranch {
	return &CachedBranch{CachedObject: ledgerState.branchStorage.Load(branchId.Bytes())}
}

// GetConsumers retrieves the approvers of a payload from the object storage.
func (ledgerState *LedgerState) GetConsumers(outputId transaction.OutputId) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	ledgerState.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputId.Bytes())

	return consumers
}

// GetAttachments retrieves the att of a payload from the object storage.
func (ledgerState *LedgerState) GetAttachments(transactionId transaction.Id) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	ledgerState.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionId.Bytes())

	return attachments
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (ledgerState *LedgerState) Shutdown() *LedgerState {
	ledgerState.storeTransactionWorkerPool.ShutdownGracefully()
	ledgerState.solidifierWorkerPool.ShutdownGracefully()

	ledgerState.transactionStorage.Shutdown()
	ledgerState.transactionMetadataStorage.Shutdown()
	ledgerState.outputStorage.Shutdown()
	ledgerState.consumerStorage.Shutdown()

	return ledgerState
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (ledgerState *LedgerState) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		ledgerState.transactionStorage,
		ledgerState.transactionMetadataStorage,
		ledgerState.outputStorage,
		ledgerState.consumerStorage,
		ledgerState.branchStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

func (ledgerState *LedgerState) storeTransactionWorker(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *tangle.CachedPayloadMetadata) {
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()

	// abort if the parameters are empty
	solidPayload := cachedPayload.Unwrap()
	if solidPayload == nil || cachedPayloadMetadata.Unwrap() == nil {
		return
	}

	// store objects in database
	cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := ledgerState.storeTransactionModels(solidPayload)

	// abort if the attachment was previously processed already (nil == was not stored)
	if cachedAttachment == nil {
		cachedTransaction.Release()
		cachedTransactionMetadata.Release()

		return
	}

	// trigger events for a new transaction
	if transactionIsNew {
		ledgerState.Events.TransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
	}

	// check solidity of transaction and its corresponding attachment
	ledgerState.solidifierWorkerPool.Submit(func() {
		ledgerState.solidifyTransactionWorker(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
	})
}

func (ledgerState *LedgerState) storeTransactionModels(solidPayload *payload.Payload) (cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment, transactionIsNew bool) {
	cachedTransaction = &transaction.CachedTransaction{CachedObject: ledgerState.transactionStorage.ComputeIfAbsent(solidPayload.Transaction().Id().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionIsNew = true

		result := solidPayload.Transaction()
		result.Persist()
		result.SetModified()

		return result
	})}

	if transactionIsNew {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: ledgerState.transactionMetadataStorage.Store(NewTransactionMetadata(solidPayload.Transaction().Id()))}

		// store references to the consumed outputs
		solidPayload.Transaction().Inputs().ForEach(func(outputId transaction.OutputId) bool {
			ledgerState.consumerStorage.Store(NewConsumer(outputId, solidPayload.Transaction().Id())).Release()

			return true
		})
	} else {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: ledgerState.transactionMetadataStorage.Load(solidPayload.Transaction().Id().Bytes())}
	}

	// store a reference from the transaction to the payload that attached it or abort, if we have processed this attachment already
	attachment, stored := ledgerState.attachmentStorage.StoreIfAbsent(NewAttachment(solidPayload.Transaction().Id(), solidPayload.Id()))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}

	return
}

func (ledgerState *LedgerState) solidifyTransactionWorker(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetdata *CachedTransactionMetadata, attachment *CachedAttachment) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedTransaction, cachedTransactionMetdata, attachment})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedTransaction, currentCachedTransactionMetadata, currentCachedAttachment := ledgerState.popElementsFromSolidificationStack(solidificationStack)
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
			if transactionSolid, err := ledgerState.isTransactionSolid(currentTransaction, currentTransactionMetadata); !transactionSolid || err != nil {
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
			ledgerState.ForEachConsumers(currentTransaction, func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment) {
				solidificationStack.PushBack([3]interface{}{cachedTransaction, transactionMetadata, cachedAttachment})
			})

			// TODO: BOOK TRANSACTION
			ledgerState.bookTransaction(cachedTransaction.Retain())
		}()
	}
}

func (ledgerState *LedgerState) popElementsFromSolidificationStack(stack *list.List) (*transaction.CachedTransaction, *CachedTransactionMetadata, *CachedAttachment) {
	currentSolidificationEntry := stack.Front()
	cachedTransaction := currentSolidificationEntry.Value.([3]interface{})[0].(*transaction.CachedTransaction)
	cachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[1].(*CachedTransactionMetadata)
	cachedAttachment := currentSolidificationEntry.Value.([3]interface{})[2].(*CachedAttachment)
	stack.Remove(currentSolidificationEntry)

	return cachedTransaction, cachedTransactionMetadata, cachedAttachment
}

func (ledgerState *LedgerState) isTransactionSolid(tx *transaction.Transaction, metadata *TransactionMetadata) (bool, error) {
	// abort if any of the models are nil or has been deleted
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return false, nil
	}

	// abort if we have previously determined the solidity status of the transaction already
	if metadata.Solid() {
		return true, nil
	}

	// get outputs that were referenced in the transaction inputs
	cachedInputs := ledgerState.getCachedOutputsFromTransactionInputs(tx)
	defer cachedInputs.Release()

	// check the solidity of the inputs and retrieve the consumed balances
	inputsSolid, consumedBalances, err := ledgerState.checkTransactionInputs(cachedInputs)

	// abort if an error occurred or the inputs are not solid, yet
	if !inputsSolid || err != nil {
		return false, err
	}

	if !ledgerState.checkTransactionOutputs(consumedBalances, tx.Outputs()) {
		return false, fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.Id())
	}

	return true, nil
}

func (ledgerState *LedgerState) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result CachedOutputs) {
	result = make(CachedOutputs)
	tx.Inputs().ForEach(func(inputId transaction.OutputId) bool {
		result[inputId] = ledgerState.GetTransactionOutput(inputId)

		return true
	})

	return
}

func (ledgerState *LedgerState) checkTransactionInputs(cachedInputs CachedOutputs) (inputsSolid bool, consumedBalances map[balance.Color]int64, err error) {
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
func (ledgerState *LedgerState) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
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
			if outputBalance.Color() == balance.COLOR_NEW {
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

func (ledgerState *LedgerState) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment)) {
	seenTransactions := make(map[transaction.Id]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		ledgerState.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
			if _, transactionSeen := seenTransactions[consumer.TransactionId()]; !transactionSeen {
				seenTransactions[consumer.TransactionId()] = types.Void

				cachedTransaction := ledgerState.GetTransaction(consumer.TransactionId())
				cachedTransactionMetadata := ledgerState.GetTransactionMetadata(consumer.TransactionId())
				for _, cachedAttachment := range ledgerState.GetAttachments(consumer.TransactionId()) {
					consume(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
				}
			}
		})

		return true
	})
}

func (ledgerState *LedgerState) bookTransaction(cachedTransaction *transaction.CachedTransaction) {
	defer cachedTransaction.Release()

	transactionToBook := cachedTransaction.Unwrap()
	if transactionToBook == nil {
		return
	}

	consumedBranches := make(map[BranchId]types.Empty)
	conflictingConsumersToFork := make(map[transaction.Id]types.Empty)
	createFork := false

	inputsSuccessfullyProcessed := transactionToBook.Inputs().ForEach(func(outputId transaction.OutputId) bool {
		cachedOutput := ledgerState.GetTransactionOutput(outputId)
		defer cachedOutput.Release()

		// abort if the output could not be found
		output := cachedOutput.Unwrap()
		if output == nil {
			return false
		}

		consumedBranches[output.BranchId()] = types.Void

		// continue if we are the first consumer and there is no double spend
		consumerCount, firstConsumerId := output.RegisterConsumer(transactionToBook.Id())
		if consumerCount == 0 {
			return true
		}

		// fork into a new branch
		createFork = true

		// also fork the previous consumer
		if consumerCount == 1 {
			conflictingConsumersToFork[firstConsumerId] = types.Void
		}

		return true
	})

	if !inputsSuccessfullyProcessed {
		return
	}

	transactionToBook.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		newOutput := NewOutput(address, transactionToBook.Id(), MasterBranchId, balances)
		newOutput.SetSolid(true)
		ledgerState.outputStorage.Store(newOutput).Release()

		return true
	})

	fmt.Println(consumedBranches)
	fmt.Println(MasterBranchId)
	fmt.Println(createFork)
}

func (ledgerState *LedgerState) InheritBranches(branches ...BranchId) (cachedAggregatedBranch *CachedBranch, err error) {
	// return the MasterBranch if we have no branches in the parameters
	if len(branches) == 0 {
		cachedAggregatedBranch = ledgerState.GetBranch(MasterBranchId)

		return
	}

	if len(branches) == 1 {
		cachedAggregatedBranch = ledgerState.GetBranch(branches[0])

		return
	}

	// filter out duplicates and shared ancestor Branches (abort if we faced an error)
	deepestCommonAncestors, err := ledgerState.findDeepestCommonAncestorBranches(branches...)
	if err != nil {
		return
	}

	// if there is only one branch that we found, then we are done
	if len(deepestCommonAncestors) == 1 {
		for _, firstBranchInList := range deepestCommonAncestors {
			cachedAggregatedBranch = firstBranchInList
		}

		return
	}

	// if there is more than one parents: aggregate
	aggregatedBranchId, aggregatedBranchParents, err := ledgerState.determineAggregatedBranchDetails(deepestCommonAncestors)
	if err != nil {
		return
	}

	newAggregatedBranchCreated := false
	cachedAggregatedBranch = &CachedBranch{CachedObject: ledgerState.branchStorage.ComputeIfAbsent(aggregatedBranchId.Bytes(), func(key []byte) (object objectstorage.StorableObject) {
		aggregatedReality := NewBranch(aggregatedBranchId, aggregatedBranchParents)

		// TODO: FIX
		/*
			for _, parentRealityId := range aggregatedBranchParents {
				tangle.GetBranch(parentRealityId).Consume(func(branch *Branch) {
					branch.RegisterSubReality(aggregatedRealityId)
				})
			}
		*/

		aggregatedReality.SetModified()

		newAggregatedBranchCreated = true

		return aggregatedReality
	})}

	if !newAggregatedBranchCreated {
		fmt.Println("1")
		// TODO: FIX
		/*
			aggregatedBranch := cachedAggregatedBranch.Unwrap()

			for _, realityId := range aggregatedBranchParents {
				if aggregatedBranch.AddParentReality(realityId) {
					tangle.GetBranch(realityId).Consume(func(branch *Branch) {
						branch.RegisterSubReality(aggregatedRealityId)
					})
				}
			}
		*/
	}

	return
}

func (ledgerState *LedgerState) determineAggregatedBranchDetails(deepestCommonAncestors CachedBranches) (aggregatedBranchId BranchId, aggregatedBranchParents []BranchId, err error) {
	aggregatedBranchParents = make([]BranchId, len(deepestCommonAncestors))

	i := 0
	aggregatedBranchConflictParents := make(CachedBranches)
	for branchId, cachedBranch := range deepestCommonAncestors {
		// release all following entries if we have encountered an error
		if err != nil {
			cachedBranch.Release()

			continue
		}

		// store BranchId as parent
		aggregatedBranchParents[i] = branchId
		i++

		// abort if we could not unwrap the Branch (should never happen)
		branch := cachedBranch.Unwrap()
		if branch == nil {
			cachedBranch.Release()

			err = fmt.Errorf("failed to unwrap brach '%s'", branchId)

			continue
		}

		if branch.IsAggregated() {
			aggregatedBranchConflictParents[branchId] = cachedBranch

			continue
		}

		err = ledgerState.collectClosestConflictAncestors(branch, aggregatedBranchConflictParents)

		cachedBranch.Release()
	}

	if err != nil {
		aggregatedBranchConflictParents.Release()
		aggregatedBranchConflictParents = nil

		return
	}

	aggregatedBranchId = ledgerState.generateAggregatedBranchId(aggregatedBranchConflictParents)

	return
}

func (ledgerState *LedgerState) generateAggregatedBranchId(aggregatedBranches CachedBranches) BranchId {
	counter := 0
	branchIds := make([]BranchId, len(aggregatedBranches))
	for branchId, cachedBranch := range aggregatedBranches {
		branchIds[counter] = branchId

		counter++

		cachedBranch.Release()
	}

	sort.Slice(branchIds, func(i, j int) bool {
		for k := 0; k < len(branchIds[k]); k++ {
			if branchIds[i][k] < branchIds[j][k] {
				return true
			} else if branchIds[i][k] > branchIds[j][k] {
				return false
			}
		}

		return false
	})

	marshalUtil := marshalutil.New(BranchIdLength * len(branchIds))
	for _, branchId := range branchIds {
		marshalUtil.WriteBytes(branchId.Bytes())
	}

	return blake2b.Sum256(marshalUtil.Bytes())
}

func (ledgerState *LedgerState) collectClosestConflictAncestors(branch *Branch, closestConflictAncestors CachedBranches) (err error) {
	// initialize stack
	stack := list.New()
	for _, parentRealityId := range branch.ParentBranches() {
		stack.PushBack(parentRealityId)
	}

	// work through stack
	processedBranches := make(map[BranchId]types.Empty)
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchId := stack.Front().Value.(BranchId)

			// abort if the parent has been processed already
			if _, branchProcessed := processedBranches[parentBranchId]; branchProcessed {
				return nil
			}
			processedBranches[parentBranchId] = types.Void

			// load parent branch from database
			cachedParentBranch := ledgerState.GetBranch(parentBranchId)

			// abort if the parent branch could not be found (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to load branch '%s'", parentBranchId)
			}

			// if the parent Branch is not aggregated, then we have found the closest conflict ancestor
			if !parentBranch.IsAggregated() {
				closestConflictAncestors[parentBranchId] = cachedParentBranch

				return nil
			}

			// queue parents for additional check (recursion)
			for _, parentRealityId := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityId)
			}

			// release the branch (we don't need it anymore)
			cachedParentBranch.Release()

			return nil
		}()

		if err != nil {
			return
		}
	}

	return
}

// findDeepestCommonAncestorBranches takes a number of BranchIds and determines the most specialized Branches (furthest
// away from the MasterBranch) in that list, that contains all of the named BranchIds.
//
// Example: If we hand in "A, B" and B has A as its parent, then the result will contain the Branch B, because B is a
//          child of A.
func (ledgerState *LedgerState) findDeepestCommonAncestorBranches(branches ...BranchId) (result CachedBranches, err error) {
	result = make(CachedBranches)

	processedBranches := make(map[BranchId]types.Empty)
	for _, branchId := range branches {
		err = func() error {
			// continue, if we have processed this branch already
			if _, exists := processedBranches[branchId]; exists {
				return nil
			}
			processedBranches[branchId] = types.Void

			// load branch from objectstorage
			cachedBranch := ledgerState.GetBranch(branchId)

			// abort if we could not load the CachedBranch
			branch := cachedBranch.Unwrap()
			if branch == nil {
				cachedBranch.Release()

				return fmt.Errorf("could not load branch '%s'", branchId)
			}

			// check branches position relative to already aggregated branches
			for aggregatedBranchId, cachedAggregatedBranch := range result {
				// abort if we can not load the branch
				aggregatedBranch := cachedAggregatedBranch.Unwrap()
				if aggregatedBranch == nil {
					return fmt.Errorf("could not load branch '%s'", aggregatedBranchId)
				}

				// if the current branch is an ancestor of an already aggregated branch, then we have found the more
				// "specialized" branch already and keep it
				if isAncestor, ancestorErr := ledgerState.branchIsAncestorOfBranch(branch, aggregatedBranch); isAncestor || ancestorErr != nil {
					return ancestorErr
				}

				// check if the aggregated Branch is an ancestor of the current Branch and abort if we face an error
				isAncestor, ancestorErr := ledgerState.branchIsAncestorOfBranch(aggregatedBranch, branch)
				if ancestorErr != nil {
					return ancestorErr
				}

				// if the aggregated branch is an ancestor of the current branch, then we have found a more specialized
				// Branch and replace the old one with this one.
				if isAncestor {
					// replace aggregated branch if we have found a more specialized on
					delete(result, aggregatedBranchId)
					cachedAggregatedBranch.Release()

					result[branchId] = cachedBranch

					return nil
				}
			}

			// store the branch as a new aggregate candidate if it was not found to be in any relation with the already
			// aggregated ones.
			result[branchId] = cachedBranch

			return nil
		}()

		// abort if an error occurred while processing the current branch
		if err != nil {
			result.Release()
			result = nil

			return
		}
	}

	return
}

func (ledgerState *LedgerState) branchIsAncestorOfBranch(ancestor *Branch, descendant *Branch) (isAncestor bool, err error) {
	if ancestor.Id() == descendant.Id() {
		return true, nil
	}

	ancestorBranches, err := ledgerState.getAncestorBranches(descendant)
	if err != nil {
		return
	}

	ancestorBranches.Consume(func(ancestorOfDescendant *Branch) {
		if ancestorOfDescendant.Id() == ancestor.Id() {
			isAncestor = true
		}
	})

	return
}

func (ledgerState *LedgerState) getAncestorBranches(branch *Branch) (ancestorBranches CachedBranches, err error) {
	// initialize result
	ancestorBranches = make(CachedBranches)

	// initialize stack
	stack := list.New()
	for _, parentRealityId := range branch.ParentBranches() {
		stack.PushBack(parentRealityId)
	}

	// work through stack
	for stack.Len() != 0 {
		// iterate through the parents (in a func so we can used defer)
		err = func() error {
			// pop parent branch id from stack
			firstStackElement := stack.Front()
			defer stack.Remove(firstStackElement)
			parentBranchId := stack.Front().Value.(BranchId)

			// abort if the parent has been processed already
			if _, branchProcessed := ancestorBranches[parentBranchId]; branchProcessed {
				return nil
			}

			// load parent branch from database
			cachedParentBranch := ledgerState.GetBranch(parentBranchId)

			// abort if the parent branch could not be founds (should never happen)
			parentBranch := cachedParentBranch.Unwrap()
			if parentBranch == nil {
				cachedParentBranch.Release()

				return fmt.Errorf("failed to unwrap branch '%s'", parentBranchId)
			}

			// store parent branch in result
			ancestorBranches[parentBranchId] = cachedParentBranch

			// queue parents for additional check (recursion)
			for _, parentRealityId := range parentBranch.ParentBranches() {
				stack.PushBack(parentRealityId)
			}

			return nil
		}()

		// abort if an error occurs while trying to process the parents
		if err != nil {
			ancestorBranches.Release()
			ancestorBranches = nil

			return
		}
	}

	return
}
