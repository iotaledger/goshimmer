package tangle

import (
	"container/list"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"
	"golang.org/x/crypto/blake2b"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingPayloadStorage  *objectstorage.ObjectStorage
	attachmentStorage      *objectstorage.ObjectStorage

	outputStorage        *objectstorage.ObjectStorage
	consumerStorage      *objectstorage.ObjectStorage
	missingOutputStorage *objectstorage.ObjectStorage
	branchStorage        *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	bookerWorkerPool       async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		// payload related storage
		payloadStorage:         osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:        osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),

		// transaction related storage
		attachmentStorage:    osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second)),
		outputStorage:        osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		missingOutputStorage: osFactory.New(osMissingOutput, osMissingOutputFactory, MissingOutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		consumerStorage:      osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second)),

		Events: *newEvents(),
	}

	return
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// GetPayload retrieves a payload from the object storage.
func (tangle *Tangle) GetPayload(payloadId payload.Id) *payload.CachedPayload {
	return &payload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetPayloadMetadata(payloadId payload.Id) *CachedPayloadMetadata {
	return &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetTransactionMetadata(transactionId transaction.Id) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: tangle.missingOutputStorage.Load(transactionId.Bytes())}
}

func (tangle *Tangle) GetTransactionOutput(outputId transaction.OutputId) *CachedOutput {
	return &CachedOutput{CachedObject: tangle.outputStorage.Load(outputId.Bytes())}
}

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetApprovers(payloadId payload.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, payloadId.Bytes())

	return approvers
}

// GetConsumers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetConsumers(outputId transaction.OutputId) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	tangle.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputId.Bytes())

	return consumers
}

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetAttachments(transactionId transaction.Id) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	tangle.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionId.Bytes())

	return attachments
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.storePayloadWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	tangle.payloadStorage.Shutdown()
	tangle.payloadMetadataStorage.Shutdown()
	tangle.approverStorage.Shutdown()
	tangle.missingPayloadStorage.Shutdown()

	return tangle
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (tangle *Tangle) Prune() error {
	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.approverStorage,
		tangle.missingPayloadStorage,
	} {
		if err := storage.Prune(); err != nil {
			return err
		}
	}

	return nil
}

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payloadToStore *payload.Payload) {
	// store the payload and transaction models
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		// abort if we have seen the payload already
		return
	}
	cachedTransactionMetadata, transactionStored := tangle.storeTransaction(payloadToStore.Transaction())

	// store the references between the different entities (we do this after the actual entities were stored, so that
	// all the metadata models exist in the database as soon as the entities are reachable by walks).
	tangle.storePayloadReferences(payloadToStore)
	if transactionStored {
		tangle.storeTransactionReferences(payloadToStore.Transaction())
	}

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadToStore.Id().Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedPayloadMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyTransactionWorker(cachedPayload, cachedPayloadMetadata, cachedTransactionMetadata)
	})
}

func (tangle *Tangle) storePayload(payloadToStore *payload.Payload) (cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, payloadStored bool) {
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore); !transactionIsNew {
		return
	} else {
		cachedPayload = &payload.CachedPayload{CachedObject: _tmp}
		cachedMetadata = &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadToStore.Id()))}
		payloadStored = true

		return
	}
}

func (tangle *Tangle) storeTransaction(tx *transaction.Transaction) (cachedTransactionMetadata *CachedTransactionMetadata, transactionStored bool) {
	cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.payloadMetadataStorage.ComputeIfAbsent(tx.Id().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionStored = true

		result := NewTransactionMetadata(tx.Id())
		result.Persist()
		result.SetModified()

		return result
	})}

	return
}

func (tangle *Tangle) storePayloadReferences(payload *payload.Payload) {
	// store trunk approver
	trunkId := payload.TrunkId()
	tangle.approverStorage.Store(NewPayloadApprover(trunkId, payload.Id())).Release()

	// store branch approver
	if branchId := payload.BranchId(); branchId != trunkId {
		tangle.approverStorage.Store(NewPayloadApprover(branchId, trunkId)).Release()
	}

	// store a reference from the transaction to the payload that attached it
	tangle.attachmentStorage.Store(NewAttachment(payload.Transaction().Id(), payload.Id()))
}

func (tangle *Tangle) storeTransactionReferences(tx *transaction.Transaction) {
	// store references to the consumed outputs
	tx.Inputs().ForEach(func(outputId transaction.OutputId) bool {
		tangle.consumerStorage.Store(NewConsumer(outputId, tx.Id()))

		return true
	})
}

func (tangle *Tangle) popElementsFromSolidificationStack(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata, *CachedTransactionMetadata) {
	currentSolidificationEntry := stack.Front()
	currentCachedPayload := currentSolidificationEntry.Value.([3]interface{})[0]
	currentCachedMetadata := currentSolidificationEntry.Value.([3]interface{})[1]
	currentCachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[2]
	stack.Remove(currentSolidificationEntry)

	return currentCachedPayload.(*payload.CachedPayload), currentCachedMetadata.(*CachedPayloadMetadata), currentCachedTransactionMetadata.(*CachedTransactionMetadata)
}

// solidifyTransactionWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyTransactionWorker(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedPayload, cachedMetadata, cachedTransactionMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedPayload, currentCachedMetadata, currentCachedTransactionMetadata := tangle.popElementsFromSolidificationStack(solidificationStack)
			defer currentCachedPayload.Release()
			defer currentCachedMetadata.Release()
			defer currentCachedTransactionMetadata.Release()

			// unwrap cached objects
			currentPayload := currentCachedPayload.Unwrap()
			currentPayloadMetadata := currentCachedMetadata.Unwrap()
			currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()

			// abort if any of the retrieved models is nil or payload is not solid
			if currentPayload == nil || currentPayloadMetadata == nil || currentTransactionMetadata == nil || !tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) {
				return
			}

			// abort if the transaction is not solid or invalid
			if transactionSolid, err := tangle.isTransactionSolid(currentPayload.Transaction(), currentTransactionMetadata); !transactionSolid || err != nil {
				if err != nil {
					// TODO: TRIGGER INVALID TX + REMOVE TXS THAT APPROVE IT
					fmt.Println(err)
				}

				return
			}

			// abort if the payload was marked as solid already (if a payload is solid already then the tx is also solid)
			if !currentPayloadMetadata.SetSolid(true) {
				return
			}

			// ... trigger solid event ...
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			// ... and schedule check of approvers
			tangle.ForeachApprovers(currentPayload.Id(), func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
				solidificationStack.PushBack([3]interface{}{payload, payloadMetadata, cachedTransactionMetadata})
			})

			// book the outputs
			if !currentTransactionMetadata.SetSolid(true) {
				return
			}

			tangle.Events.TransactionSolid.Trigger(currentPayload.Transaction(), currentTransactionMetadata)

			tangle.ForEachConsumers(currentPayload.Transaction(), func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
				solidificationStack.PushBack([3]interface{}{payload, payloadMetadata, cachedTransactionMetadata})
			})

			payloadToBook := cachedPayload.Retain()
			tangle.bookerWorkerPool.Submit(func() {
				tangle.bookPayloadTransaction(payloadToBook)
			})
		}()
	}
}

func (tangle *Tangle) bookPayloadTransaction(cachedPayload *payload.CachedPayload) {
	payloadToBook := cachedPayload.Unwrap()
	defer cachedPayload.Release()

	if payloadToBook == nil {
		return
	}
	transactionToBook := payloadToBook.Transaction()

	consumedBranches := make(map[BranchId]types.Empty)
	conflictingConsumersToFork := make(map[transaction.Id]types.Empty)
	createFork := false

	inputsSuccessfullyProcessed := payloadToBook.Transaction().Inputs().ForEach(func(outputId transaction.OutputId) bool {
		cachedOutput := tangle.GetTransactionOutput(outputId)
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
		tangle.outputStorage.Store(newOutput)

		return true
	})

	fmt.Println(consumedBranches)
	fmt.Println(MasterBranchId)
	fmt.Println(createFork)
}

func (tangle *Tangle) InheritBranches(branches ...BranchId) (cachedAggregatedBranch *CachedBranch, err error) {
	// return the MasterBranch if we have no branches in the parameters
	if len(branches) == 0 {
		cachedAggregatedBranch = tangle.GetBranch(MasterBranchId)

		return
	}

	if len(branches) == 1 {
		cachedAggregatedBranch = tangle.GetBranch(branches[0])

		return
	}

	// filter out duplicates and shared ancestor Branches (abort if we faced an error)
	deepestCommonAncestors, err := tangle.findDeepestCommonAncestorBranches(branches...)
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
	aggregatedBranchId, aggregatedBranchParents, err := tangle.determineAggregatedBranchDetails(deepestCommonAncestors)
	if err != nil {
		return
	}

	newAggregatedBranchCreated := false
	cachedAggregatedBranch = &CachedBranch{CachedObject: tangle.branchStorage.ComputeIfAbsent(aggregatedBranchId.Bytes(), func(key []byte) (object objectstorage.StorableObject) {
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

func (tangle *Tangle) determineAggregatedBranchDetails(deepestCommonAncestors CachedBranches) (aggregatedBranchId BranchId, aggregatedBranchParents []BranchId, err error) {
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

		err = tangle.collectClosestConflictAncestors(branch, aggregatedBranchConflictParents)

		cachedBranch.Release()
	}

	if err != nil {
		aggregatedBranchConflictParents.Release()
		aggregatedBranchConflictParents = nil

		return
	}

	aggregatedBranchId = tangle.generateAggregatedBranchId(aggregatedBranchConflictParents)

	return
}

func (tangle *Tangle) generateAggregatedBranchId(aggregatedBranches CachedBranches) BranchId {
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

func (tangle *Tangle) collectClosestConflictAncestors(branch *Branch, closestConflictAncestors CachedBranches) (err error) {
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
			cachedParentBranch := tangle.GetBranch(parentBranchId)

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
func (tangle *Tangle) findDeepestCommonAncestorBranches(branches ...BranchId) (result CachedBranches, err error) {
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
			cachedBranch := tangle.GetBranch(branchId)

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
				if isAncestor, ancestorErr := tangle.branchIsAncestorOfBranch(branch, aggregatedBranch); isAncestor || ancestorErr != nil {
					return ancestorErr
				}

				// check if the aggregated Branch is an ancestor of the current Branch and abort if we face an error
				isAncestor, ancestorErr := tangle.branchIsAncestorOfBranch(aggregatedBranch, branch)
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

func (tangle *Tangle) branchIsAncestorOfBranch(ancestor *Branch, descendant *Branch) (isAncestor bool, err error) {
	if ancestor.Id() == descendant.Id() {
		return true, nil
	}

	ancestorBranches, err := tangle.getAncestorBranches(descendant)
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

func (tangle *Tangle) getAncestorBranches(branch *Branch) (ancestorBranches CachedBranches, err error) {
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
			cachedParentBranch := tangle.GetBranch(parentBranchId)

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

func (tangle *Tangle) GetBranch(branchId BranchId) *CachedBranch {
	// TODO: IMPLEMENT
	return nil
}

func (tangle *Tangle) ForeachApprovers(payloadId payload.Id, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata)) {
	tangle.GetApprovers(payloadId).Consume(func(approver *PayloadApprover) {
		approvingPayloadId := approver.GetApprovingPayloadId()
		approvingCachedPayload := tangle.GetPayload(approvingPayloadId)

		approvingCachedPayload.Consume(func(payload *payload.Payload) {
			consume(approvingCachedPayload, tangle.GetPayloadMetadata(approvingPayloadId), tangle.GetTransactionMetadata(payload.Transaction().Id()))
		})
	})
}

func (tangle *Tangle) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata)) {
	seenTransactions := make(map[transaction.Id]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		tangle.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
			// keep track of the processed transactions (the same transaction can consume multiple outputs)
			if _, transactionSeen := seenTransactions[consumer.TransactionId()]; transactionSeen {
				seenTransactions[consumer.TransactionId()] = types.Void

				transactionMetadata := tangle.GetTransactionMetadata(consumer.TransactionId())

				// retrieve all the payloads that attached the transaction
				tangle.GetAttachments(consumer.TransactionId()).Consume(func(attachment *Attachment) {
					consume(tangle.GetPayload(attachment.PayloadId()), tangle.GetPayloadMetadata(attachment.PayloadId()), transactionMetadata)
				})
			}
		})

		return true
	})
}

// isPayloadSolid returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) isPayloadSolid(payload *payload.Payload, metadata *PayloadMetadata) bool {
	if payload == nil || payload.IsDeleted() {
		return false
	}

	if metadata == nil || metadata.IsDeleted() {
		return false
	}

	if metadata.IsSolid() {
		return true
	}

	return tangle.isPayloadMarkedAsSolid(payload.TrunkId()) && tangle.isPayloadMarkedAsSolid(payload.BranchId())
}

// isPayloadMarkedAsSolid returns true if the payload was marked as solid already (by setting the corresponding flags
// in its metadata.
func (tangle *Tangle) isPayloadMarkedAsSolid(payloadId payload.Id) bool {
	if payloadId == payload.GenesisId {
		return true
	}

	transactionMetadataCached := tangle.GetPayloadMetadata(payloadId)
	if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
		transactionMetadataCached.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(NewMissingPayload(payloadId)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*MissingPayload).GetId())
			})
		}

		return false
	} else if !transactionMetadata.IsSolid() {
		transactionMetadataCached.Release()

		return false
	}
	transactionMetadataCached.Release()

	return true
}

func (tangle *Tangle) isTransactionSolid(tx *transaction.Transaction, metadata *TransactionMetadata) (bool, error) {
	// abort if any of the models are nil or has been deleted
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return false, nil
	}

	// abort if we have previously determined the solidity status of the transaction already
	if metadata.Solid() {
		return true, nil
	}

	// get outputs that were referenced in the transaction inputs
	cachedInputs := tangle.getCachedOutputsFromTransactionInputs(tx)
	defer cachedInputs.Release()

	// check the solidity of the inputs and retrieve the consumed balances
	inputsSolid, consumedBalances, err := tangle.checkTransactionInputs(cachedInputs)

	// abort if an error occurred or the inputs are not solid, yet
	if !inputsSolid || err != nil {
		return false, err
	}

	if !tangle.checkTransactionOutputs(consumedBalances, tx.Outputs()) {
		return false, fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.Id())
	}

	return true, nil
}

func (tangle *Tangle) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result CachedOutputs) {
	result = make(CachedOutputs)
	tx.Inputs().ForEach(func(inputId transaction.OutputId) bool {
		result[inputId] = tangle.GetTransactionOutput(inputId)

		return true
	})

	return
}

func (tangle *Tangle) checkTransactionInputs(cachedInputs CachedOutputs) (inputsSolid bool, consumedBalances map[balance.Color]int64, err error) {
	inputsSolid = true
	consumedBalances = make(map[balance.Color]int64)

	for inputId, cachedInput := range cachedInputs {
		if !cachedInput.Exists() {
			inputsSolid = false

			if cachedMissingOutput, missingOutputStored := tangle.missingOutputStorage.StoreIfAbsent(NewMissingOutput(inputId)); missingOutputStored {
				cachedMissingOutput.Consume(func(object objectstorage.StorableObject) {
					tangle.Events.OutputMissing.Trigger(object.(*MissingOutput).Id())
				})
			}

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
func (tangle *Tangle) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
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
