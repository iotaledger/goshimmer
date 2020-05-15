package tangle

import (
	"container/list"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/address"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/balance"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	branchManager *branchmanager.BranchManager

	payloadStorage             *objectstorage.ObjectStorage
	payloadMetadataStorage     *objectstorage.ObjectStorage
	approverStorage            *objectstorage.ObjectStorage
	missingPayloadStorage      *objectstorage.ObjectStorage
	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	attachmentStorage          *objectstorage.ObjectStorage
	outputStorage              *objectstorage.ObjectStorage
	consumerStorage            *objectstorage.ObjectStorage

	Events Events

	workerPool        async.WorkerPool
	cleanupWorkerPool async.WorkerPool
}

// New is the constructor of a Tangle and creates a new Tangle object from the given details.
func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		branchManager: branchmanager.New(badgerInstance),

		payloadStorage:             osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage:     osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:      osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:            osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IDLength, payload.IDLength), objectstorage.KeysOnly(true)),
		transactionStorage:         osFactory.New(osTransaction, osTransactionFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		transactionMetadataStorage: osFactory.New(osTransactionMetadata, osTransactionMetadataFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		attachmentStorage:          osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		outputStorage:              osFactory.New(osOutput, osOutputFactory, OutputKeyPartitions, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		consumerStorage:            osFactory.New(osConsumer, osConsumerFactory, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second), osLeakDetectionOption),

		Events: *newEvents(),
	}

	return
}

// BranchManager is the getter for the manager that takes care of creating and updating branches.
func (tangle *Tangle) BranchManager() *branchmanager.BranchManager {
	return tangle.branchManager
}

// Transaction loads the given transaction from the objectstorage.
func (tangle *Tangle) Transaction(transactionID transaction.ID) *transaction.CachedTransaction {
	return &transaction.CachedTransaction{CachedObject: tangle.transactionStorage.Load(transactionID.Bytes())}
}

// TransactionMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) TransactionMetadata(transactionID transaction.ID) *CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(transactionID.Bytes())}
}

// TransactionOutput loads the given output from the objectstorage.
func (tangle *Tangle) TransactionOutput(outputID transaction.OutputID) *CachedOutput {
	return &CachedOutput{CachedObject: tangle.outputStorage.Load(outputID.Bytes())}
}

// Consumers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) Consumers(outputID transaction.OutputID) CachedConsumers {
	consumers := make(CachedConsumers, 0)
	tangle.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputID.Bytes())

	return consumers
}

// Attachments retrieves the attachment of a payload from the object storage.
func (tangle *Tangle) Attachments(transactionID transaction.ID) CachedAttachments {
	attachments := make(CachedAttachments, 0)
	tangle.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionID.Bytes())

	return attachments
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.workerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// Payload retrieves a payload from the object storage.
func (tangle *Tangle) Payload(payloadID payload.ID) *payload.CachedPayload {
	return &payload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadID.Bytes())}
}

// PayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) PayloadMetadata(payloadID payload.ID) *CachedPayloadMetadata {
	return &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadID.Bytes())}
}

// Approvers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) Approvers(payloadID payload.ID) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, payloadID.Bytes())

	return approvers
}

// Shutdown stops the worker pools and shuts down the object storage instances.
func (tangle *Tangle) Shutdown() *Tangle {
	tangle.workerPool.ShutdownGracefully()
	tangle.cleanupWorkerPool.ShutdownGracefully()

	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.missingPayloadStorage,
		tangle.approverStorage,
		tangle.transactionStorage,
		tangle.transactionMetadataStorage,
		tangle.attachmentStorage,
		tangle.outputStorage,
		tangle.consumerStorage,
	} {
		storage.Shutdown()
	}

	return tangle
}

// Prune resets the database and deletes all objects (for testing or "node resets").
func (tangle *Tangle) Prune() (err error) {
	if err = tangle.branchManager.Prune(); err != nil {
		return
	}

	for _, storage := range []*objectstorage.ObjectStorage{
		tangle.payloadStorage,
		tangle.payloadMetadataStorage,
		tangle.missingPayloadStorage,
		tangle.approverStorage,
		tangle.transactionStorage,
		tangle.transactionMetadataStorage,
		tangle.attachmentStorage,
		tangle.outputStorage,
		tangle.consumerStorage,
	} {
		if err = storage.Prune(); err != nil {
			return
		}
	}

	return
}

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payloadToStore *payload.Payload) {
	// store the payload models or abort if we have seen the payload already
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		return
	}
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()

	// store transaction models or abort if we have seen this attachment already  (nil == was not stored)
	cachedTransaction, cachedTransactionMetadata, cachedAttachment, transactionIsNew := tangle.storeTransactionModels(payloadToStore)
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()
	if cachedAttachment == nil {
		return
	}
	defer cachedAttachment.Release()

	// store the references between the different entities (we do this after the actual entities were stored, so that
	// all the metadata models exist in the database as soon as the entities are reachable by walks).
	tangle.storePayloadReferences(payloadToStore)

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadToStore.ID().Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedPayloadMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)
	if transactionIsNew {
		tangle.Events.TransactionReceived.Trigger(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
	}

	// check solidity
	tangle.solidifyPayload(cachedPayload.Retain(), cachedPayloadMetadata.Retain(), cachedTransaction.Retain(), cachedTransactionMetadata.Retain())
}

func (tangle *Tangle) storePayload(payloadToStore *payload.Payload) (cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, payloadStored bool) {
	storedTransaction, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore)
	if !transactionIsNew {
		return
	}

	cachedPayload = &payload.CachedPayload{CachedObject: storedTransaction}
	cachedMetadata = &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadToStore.ID()))}
	payloadStored = true

	return
}

func (tangle *Tangle) storeTransactionModels(solidPayload *payload.Payload) (cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment, transactionIsNew bool) {
	cachedTransaction = &transaction.CachedTransaction{CachedObject: tangle.transactionStorage.ComputeIfAbsent(solidPayload.Transaction().ID().Bytes(), func(key []byte) objectstorage.StorableObject {
		transactionIsNew = true

		result := solidPayload.Transaction()
		result.Persist()
		result.SetModified()

		return result
	})}

	if transactionIsNew {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Store(NewTransactionMetadata(solidPayload.Transaction().ID()))}

		// store references to the consumed outputs
		solidPayload.Transaction().Inputs().ForEach(func(outputId transaction.OutputID) bool {
			tangle.consumerStorage.Store(NewConsumer(outputId, solidPayload.Transaction().ID())).Release()

			return true
		})
	} else {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(solidPayload.Transaction().ID().Bytes())}
	}

	// store a reference from the transaction to the payload that attached it or abort, if we have processed this attachment already
	attachment, stored := tangle.attachmentStorage.StoreIfAbsent(NewAttachment(solidPayload.Transaction().ID(), solidPayload.ID()))
	if !stored {
		return
	}
	cachedAttachment = &CachedAttachment{CachedObject: attachment}

	return
}

func (tangle *Tangle) storePayloadReferences(payload *payload.Payload) {
	// store trunk approver
	trunkID := payload.TrunkID()
	tangle.approverStorage.Store(NewPayloadApprover(trunkID, payload.ID())).Release()

	// store branch approver
	if branchID := payload.BranchID(); branchID != trunkID {
		tangle.approverStorage.Store(NewPayloadApprover(branchID, trunkID)).Release()
	}
}

func (tangle *Tangle) popElementsFromSolidificationStack(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata, *transaction.CachedTransaction, *CachedTransactionMetadata) {
	currentSolidificationEntry := stack.Front()
	currentCachedPayload := currentSolidificationEntry.Value.([4]interface{})[0].(*payload.CachedPayload)
	currentCachedMetadata := currentSolidificationEntry.Value.([4]interface{})[1].(*CachedPayloadMetadata)
	currentCachedTransaction := currentSolidificationEntry.Value.([4]interface{})[2].(*transaction.CachedTransaction)
	currentCachedTransactionMetadata := currentSolidificationEntry.Value.([4]interface{})[3].(*CachedTransactionMetadata)
	stack.Remove(currentSolidificationEntry)

	return currentCachedPayload, currentCachedMetadata, currentCachedTransaction, currentCachedTransactionMetadata
}

// solidifyPayload is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyPayload(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([4]interface{}{cachedPayload, cachedMetadata, cachedTransaction, cachedTransactionMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// retrieve cached objects
		currentCachedPayload, currentCachedMetadata, currentCachedTransaction, currentCachedTransactionMetadata := tangle.popElementsFromSolidificationStack(solidificationStack)

		// unwrap cached objects
		currentPayload := currentCachedPayload.Unwrap()
		currentPayloadMetadata := currentCachedMetadata.Unwrap()
		currentTransaction := currentCachedTransaction.Unwrap()
		currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()

		// abort if any of the retrieved models are nil
		if currentPayload == nil || currentPayloadMetadata == nil || currentTransaction == nil || currentTransactionMetadata == nil {
			currentCachedPayload.Release()
			currentCachedMetadata.Release()
			currentCachedTransaction.Release()
			currentCachedTransactionMetadata.Release()

			return
		}

		// abort if the transaction is not solid or invalid
		transactionSolid, consumedBranches, err := tangle.checkTransactionSolidity(currentTransaction, currentTransactionMetadata)
		if err != nil || !transactionSolid {
			if err != nil {
				// TODO: TRIGGER INVALID TX + REMOVE TXS + PAYLOADS THAT APPROVE IT
				fmt.Println(err, currentTransaction)
			}

			currentCachedPayload.Release()
			currentCachedMetadata.Release()
			currentCachedTransaction.Release()
			currentCachedTransactionMetadata.Release()

			return
		}

		// abort if the payload is not solid or invalid
		payloadSolid, err := tangle.checkPayloadSolidity(currentPayload, currentPayloadMetadata, consumedBranches)
		if err != nil || !payloadSolid {
			if err != nil {
				// TODO: TRIGGER INVALID TX + REMOVE TXS + PAYLOADS THAT APPROVE IT
				fmt.Println(err, currentTransaction)
			}

			currentCachedPayload.Release()
			currentCachedMetadata.Release()
			currentCachedTransaction.Release()
			currentCachedTransactionMetadata.Release()

			return
		}

		// book the solid entities
		transactionBooked, payloadBooked, bookingErr := tangle.book(currentCachedPayload.Retain(), currentCachedMetadata.Retain(), currentCachedTransaction.Retain(), currentCachedTransactionMetadata.Retain())
		if bookingErr != nil {
			tangle.Events.Error.Trigger(bookingErr)

			currentCachedPayload.Release()
			currentCachedMetadata.Release()
			currentCachedTransaction.Release()
			currentCachedTransactionMetadata.Release()

			return
		}

		if transactionBooked {
			tangle.ForEachConsumers(currentTransaction, func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment) {
				solidificationStack.PushBack([3]interface{}{cachedTransaction, transactionMetadata, cachedAttachment})
			})
		}

		if payloadBooked {
			// ... and schedule check of approvers
			tangle.ForeachApprovers(currentPayload.ID(), func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata) {
				solidificationStack.PushBack([4]interface{}{payload, payloadMetadata, transaction, transactionMetadata})
			})
		}

		currentCachedPayload.Release()
		currentCachedMetadata.Release()
		currentCachedTransaction.Release()
		currentCachedTransactionMetadata.Release()
	}
}

func (tangle *Tangle) book(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) (transactionBooked bool, payloadBooked bool, err error) {
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	if transactionBooked, err = tangle.bookTransaction(cachedTransaction.Retain(), cachedTransactionMetadata.Retain()); err != nil {
		return
	}

	if payloadBooked, err = tangle.bookPayload(cachedPayload.Retain(), cachedPayloadMetadata.Retain(), cachedTransactionMetadata.Retain()); err != nil {
		return
	}

	return
}

func (tangle *Tangle) bookTransaction(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata) (transactionBooked bool, err error) {
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	transactionToBook := cachedTransaction.Unwrap()
	if transactionToBook == nil {
		// TODO: explicit error var
		err = errors.New("failed to unwrap transaction")

		return
	}

	transactionMetadata := cachedTransactionMetadata.Unwrap()
	if transactionMetadata == nil {
		// TODO: explicit error var
		err = errors.New("failed to unwrap transaction metadata")

		return
	}

	// abort if this transaction was booked by another process already
	if !transactionMetadata.SetSolid(true) {
		return
	}

	consumedBranches := make(branchmanager.BranchIds)
	conflictingInputs := make([]transaction.OutputID, 0)
	conflictingInputsOfFirstConsumers := make(map[transaction.ID][]transaction.OutputID)

	if !transactionToBook.Inputs().ForEach(func(outputID transaction.OutputID) bool {
		cachedOutput := tangle.TransactionOutput(outputID)
		defer cachedOutput.Release()

		// abort if the output could not be found
		output := cachedOutput.Unwrap()
		if output == nil {
			err = fmt.Errorf("could not load output '%s'", outputID)

			return false
		}

		consumedBranches[output.BranchID()] = types.Void

		// register the current consumer and check if the input has been consumed before
		consumerCount, firstConsumerID := output.RegisterConsumer(transactionToBook.ID())
		switch consumerCount {
		// continue if we are the first consumer and there is no double spend
		case 0:
			return true

		// if the input has been consumed before but not been forked, yet
		case 1:
			// keep track of the conflicting inputs so we can fork them
			if _, conflictingInputsExist := conflictingInputsOfFirstConsumers[firstConsumerID]; !conflictingInputsExist {
				conflictingInputsOfFirstConsumers[firstConsumerID] = make([]transaction.OutputID, 0)
			}
			conflictingInputsOfFirstConsumers[firstConsumerID] = append(conflictingInputsOfFirstConsumers[firstConsumerID], outputID)
		}

		return true
	}) {
		return
	}

	cachedTargetBranch, err := tangle.branchManager.AggregateBranches(consumedBranches.ToList()...)
	if err != nil {
		return
	}
	defer cachedTargetBranch.Release()

	targetBranch := cachedTargetBranch.Unwrap()
	if targetBranch == nil {
		err = errors.New("failed to unwrap target branch")

		return
	}
	targetBranch.Persist()

	if len(conflictingInputs) >= 1 {
		cachedTargetBranch, _ = tangle.branchManager.Fork(branchmanager.NewBranchID(transactionToBook.ID()), []branchmanager.BranchID{targetBranch.ID()}, conflictingInputs)
		defer cachedTargetBranch.Release()

		targetBranch = cachedTargetBranch.Unwrap()
		if targetBranch == nil {
			err = errors.New("failed to inherit branches")

			return
		}
	}

	// book transaction into target branch
	transactionMetadata.SetBranchID(targetBranch.ID())

	// book outputs into the target branch
	transactionToBook.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		newOutput := NewOutput(address, transactionToBook.ID(), targetBranch.ID(), balances)
		newOutput.SetSolid(true)
		tangle.outputStorage.Store(newOutput).Release()

		return true
	})

	// fork the conflicting transactions into their own branch
	decisionPending := false
	for consumerID, conflictingInputs := range conflictingInputsOfFirstConsumers {
		_, decisionFinalized, forkedErr := tangle.Fork(consumerID, conflictingInputs)
		if forkedErr != nil {
			err = forkedErr

			return
		}

		decisionPending = decisionPending || !decisionFinalized
	}

	// trigger events
	tangle.Events.TransactionBooked.Trigger(cachedTransaction, cachedTransactionMetadata, cachedTargetBranch, conflictingInputs, decisionPending)

	transactionBooked = true

	return
}

func (tangle *Tangle) bookPayload(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) (payloadBooked bool, err error) {
	defer cachedPayload.Release()
	defer cachedPayloadMetadata.Release()
	defer cachedTransactionMetadata.Release()

	valueObject := cachedPayload.Unwrap()
	valueObjectMetadata := cachedPayloadMetadata.Unwrap()
	transactionMetadata := cachedTransactionMetadata.Unwrap()

	if valueObject == nil || valueObjectMetadata == nil || transactionMetadata == nil {
		return
	}

	branchBranchID := tangle.payloadBranchID(valueObject.BranchID())
	trunkBranchID := tangle.payloadBranchID(valueObject.TrunkID())
	transactionBranchID := transactionMetadata.BranchID()

	if branchBranchID == branchmanager.UndefinedBranchID || trunkBranchID == branchmanager.UndefinedBranchID || transactionBranchID == branchmanager.UndefinedBranchID {
		return
	}

	cachedAggregatedBranch, err := tangle.BranchManager().AggregateBranches([]branchmanager.BranchID{branchBranchID, trunkBranchID, transactionBranchID}...)
	if err != nil {
		return
	}
	defer cachedAggregatedBranch.Release()

	aggregatedBranch := cachedAggregatedBranch.Unwrap()
	if aggregatedBranch == nil {
		return
	}

	payloadBooked = valueObjectMetadata.SetBranchID(aggregatedBranch.ID())

	return
}

// ForeachApprovers iterates through the approvers of a payload and calls the passed in consumer function.
func (tangle *Tangle) ForeachApprovers(payloadID payload.ID, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata, transaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata)) {
	tangle.Approvers(payloadID).Consume(func(approver *PayloadApprover) {
		approvingPayloadID := approver.ApprovingPayloadID()
		approvingCachedPayload := tangle.Payload(approvingPayloadID)

		approvingCachedPayload.Consume(func(payload *payload.Payload) {
			consume(approvingCachedPayload, tangle.PayloadMetadata(approvingPayloadID), tangle.Transaction(payload.Transaction().ID()), tangle.TransactionMetadata(payload.Transaction().ID()))
		})
	})
}

// payloadBranchID returns the BranchID that the referenced Payload was booked into.
func (tangle *Tangle) payloadBranchID(payloadID payload.ID) branchmanager.BranchID {
	if payloadID == payload.GenesisID {
		return branchmanager.MasterBranchID
	}

	cachedPayloadMetadata := tangle.PayloadMetadata(payloadID)
	defer cachedPayloadMetadata.Release()

	payloadMetadata := cachedPayloadMetadata.Unwrap()
	if payloadMetadata == nil {
		cachedPayloadMetadata.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(NewMissingPayload(payloadID)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*MissingPayload).ID())
			})
		}

		return branchmanager.UndefinedBranchID
	}

	// the BranchID is only set if the payload was also marked as solid
	return payloadMetadata.BranchID()
}

// checkPayloadSolidity returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) checkPayloadSolidity(payload *payload.Payload, payloadMetadata *PayloadMetadata, transactionBranches []branchmanager.BranchID) (solid bool, err error) {
	if payload == nil || payload.IsDeleted() || payloadMetadata == nil || payloadMetadata.IsDeleted() {
		return
	}

	if solid = payloadMetadata.IsSolid(); solid {
		return
	}

	combinedBranches := transactionBranches

	trunkBranchID := tangle.payloadBranchID(payload.TrunkID())
	if trunkBranchID == branchmanager.UndefinedBranchID {
		return
	}
	combinedBranches = append(combinedBranches, trunkBranchID)

	branchBranchID := tangle.payloadBranchID(payload.BranchID())
	if branchBranchID == branchmanager.UndefinedBranchID {
		return
	}
	combinedBranches = append(combinedBranches, branchBranchID)

	branchesConflicting, err := tangle.branchManager.BranchesConflicting(combinedBranches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the payload '%s' combines conflicting versions of the ledger state", payload.ID())

		return
	}

	solid = true

	return
}

func (tangle *Tangle) checkTransactionSolidity(tx *transaction.Transaction, metadata *TransactionMetadata) (solid bool, consumedBranches []branchmanager.BranchID, err error) {
	// abort if any of the models are nil or has been deleted
	if tx == nil || tx.IsDeleted() || metadata == nil || metadata.IsDeleted() {
		return
	}

	// abort if we have previously determined the solidity status of the transaction already
	if solid = metadata.Solid(); solid {
		consumedBranches = []branchmanager.BranchID{metadata.BranchID()}

		return
	}

	// determine the consumed inputs and balances of the transaction
	inputsSolid, cachedInputs, consumedBalances, consumedBranchesMap, err := tangle.retrieveConsumedInputDetails(tx)
	if err != nil || !inputsSolid {
		return
	}
	defer cachedInputs.Release()

	// abort if the outputs are not matching the inputs
	if !tangle.checkTransactionOutputs(consumedBalances, tx.Outputs()) {
		err = fmt.Errorf("the outputs do not match the inputs in transaction with id '%s'", tx.ID())

		return
	}

	// abort if the branches are conflicting or we faced an error when checking the validity
	consumedBranches = consumedBranchesMap.ToList()
	branchesConflicting, err := tangle.branchManager.BranchesConflicting(consumedBranches...)
	if err != nil {
		return
	}
	if branchesConflicting {
		err = fmt.Errorf("the transaction '%s' spends conflicting inputs", tx.ID())

		return
	}

	// set the result to be solid and valid
	solid = true

	return
}

// LoadSnapshot creates a set of outputs in the value tangle, that are forming the genesis for future transactions.
func (tangle *Tangle) LoadSnapshot(snapshot map[transaction.ID]map[address.Address][]*balance.Balance) {
	for transactionID, addressBalances := range snapshot {
		for outputAddress, balances := range addressBalances {
			input := NewOutput(outputAddress, transactionID, branchmanager.MasterBranchID, balances)
			input.SetSolid(true)

			// store output and abort if the snapshot has already been loaded earlier (output exists in the database)
			cachedOutput, stored := tangle.outputStorage.StoreIfAbsent(input)
			if !stored {
				return
			}

			cachedOutput.Release()
		}
	}
}

// OutputsOnAddress retrieves all the Outputs that are associated with an address.
func (tangle *Tangle) OutputsOnAddress(address address.Address) (result CachedOutputs) {
	result = make(CachedOutputs)
	tangle.outputStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		outputID, _, err := transaction.OutputIDFromBytes(key)
		if err != nil {
			panic(err)
		}

		result[outputID] = &CachedOutput{CachedObject: cachedObject}

		return true
	}, address.Bytes())

	return
}

func (tangle *Tangle) getCachedOutputsFromTransactionInputs(tx *transaction.Transaction) (result CachedOutputs) {
	result = make(CachedOutputs)
	tx.Inputs().ForEach(func(inputId transaction.OutputID) bool {
		result[inputId] = tangle.TransactionOutput(inputId)

		return true
	})

	return
}

func (tangle *Tangle) retrieveConsumedInputDetails(tx *transaction.Transaction) (inputsSolid bool, cachedInputs CachedOutputs, consumedBalances map[balance.Color]int64, consumedBranches branchmanager.BranchIds, err error) {
	cachedInputs = tangle.getCachedOutputsFromTransactionInputs(tx)
	consumedBalances = make(map[balance.Color]int64)
	consumedBranches = make(branchmanager.BranchIds)
	for _, cachedInput := range cachedInputs {
		input := cachedInput.Unwrap()
		if input == nil || !input.Solid() {
			cachedInputs.Release()

			return
		}

		consumedBranches[input.BranchID()] = types.Void

		// calculate the input balances
		for _, inputBalance := range input.Balances() {
			var newBalance int64
			if currentBalance, balanceExists := consumedBalances[inputBalance.Color()]; balanceExists {
				// check overflows in the numbers
				if inputBalance.Value() > math.MaxInt64-currentBalance {
					// TODO: make it an explicit error var
					err = fmt.Errorf("buffer overflow in balances of inputs")

					cachedInputs.Release()

					return
				}

				newBalance = currentBalance + inputBalance.Value()
			} else {
				newBalance = inputBalance.Value()
			}
			consumedBalances[inputBalance.Color()] = newBalance
		}
	}
	inputsSolid = true

	return
}

// checkTransactionOutputs is a utility function that returns true, if the outputs are consuming all of the given inputs
// (the sum of all the balance changes is 0). It also accounts for the ability to "recolor" coins during the creating of
// outputs. If this function returns false, then the outputs that are defined in the transaction are invalid and the
// transaction should be removed from the ledger state.
func (tangle *Tangle) checkTransactionOutputs(inputBalances map[balance.Color]int64, outputs *transaction.Outputs) bool {
	// create a variable to keep track of outputs that create a new color
	var newlyColoredCoins int64
	var uncoloredCoins int64

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

			// sidestep logic if we have a newly colored output (we check the supply later)
			if outputBalance.Color() == balance.ColorIOTA {
				// catch overflows
				if uncoloredCoins > math.MaxInt64-outputBalance.Value() {
					return false
				}

				uncoloredCoins += outputBalance.Value()

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

			// subtract the spent coins from the supply of this color
			inputBalances[outputBalance.Color()] -= outputBalance.Value()

			// cleanup empty map entries (we have exhausted our funds)
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

	// the outputs are valid if they spend all consumed funds
	return unspentCoins == newlyColoredCoins+uncoloredCoins
}

// Fork creates a new branch from an existing transaction.
func (tangle *Tangle) Fork(transactionID transaction.ID, conflictingInputs []transaction.OutputID) (forked bool, finalized bool, err error) {
	cachedTransaction := tangle.Transaction(transactionID)
	cachedTransactionMetadata := tangle.TransactionMetadata(transactionID)
	defer cachedTransaction.Release()
	defer cachedTransactionMetadata.Release()

	tx := cachedTransaction.Unwrap()
	if tx == nil {
		err = fmt.Errorf("failed to load transaction '%s'", transactionID)

		return
	}
	txMetadata := cachedTransactionMetadata.Unwrap()
	if txMetadata == nil {
		err = fmt.Errorf("failed to load metadata of transaction '%s'", transactionID)

		return
	}

	// abort if this transaction was finalized already
	if txMetadata.Finalized() {
		finalized = true

		return
	}

	// update / create new branch
	cachedTargetBranch, newBranchCreated := tangle.branchManager.Fork(branchmanager.NewBranchID(tx.ID()), []branchmanager.BranchID{txMetadata.BranchID()}, conflictingInputs)
	defer cachedTargetBranch.Release()

	// abort if the branch existed already
	if !newBranchCreated {
		return
	}

	// unpack branch
	targetBranch := cachedTargetBranch.Unwrap()
	if targetBranch == nil {
		err = fmt.Errorf("failed to unpack branch for transaction '%s'", transactionID)

		return
	}

	// move transactions to new branch
	if err = tangle.moveTransactionToBranch(cachedTransaction.Retain(), cachedTransactionMetadata.Retain(), cachedTargetBranch.Retain()); err != nil {
		return
	}

	// trigger events + set result
	tangle.Events.Fork.Trigger(cachedTransaction, cachedTransactionMetadata, targetBranch, conflictingInputs)
	forked = true

	return
}

// TODO: write comment what it does
func (tangle *Tangle) moveTransactionToBranch(cachedTransaction *transaction.CachedTransaction, cachedTransactionMetadata *CachedTransactionMetadata, cachedTargetBranch *branchmanager.CachedBranch) (err error) {
	// push transaction that shall be moved to the stack
	transactionStack := list.New()
	branchStack := list.New()
	branchStack.PushBack([3]interface{}{cachedTransactionMetadata.Unwrap().BranchID(), cachedTargetBranch, transactionStack})
	transactionStack.PushBack([2]interface{}{cachedTransaction, cachedTransactionMetadata})

	// iterate through all transactions (grouped by their branch)
	for branchStack.Len() >= 1 {
		if err = func() error {
			// retrieve branch details from stack
			currentSolidificationEntry := branchStack.Front()
			currentSourceBranch := currentSolidificationEntry.Value.([3]interface{})[0].(branchmanager.BranchID)
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
					if currentTransactionMetadata.BranchID() != currentSourceBranch {
						// abort if we the branch is a conflict branch or an error occurred while trying to elevate
						isConflictBranch, _, elevateErr := tangle.branchManager.ElevateConflictBranch(currentTransactionMetadata.BranchID(), targetBranch.ID())
						if elevateErr != nil || isConflictBranch {
							return elevateErr
						}

						// determine the new branch of the transaction
						newCachedTargetBranch, branchErr := tangle.calculateBranchOfTransaction(currentTransaction)
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
						branchStack.PushBack([3]interface{}{currentTransactionMetadata.BranchID(), newCachedTargetBranch.Retain(), newTransactionStack})

						return nil
					}

					// abort if we did not modify the branch of the transaction
					if !currentTransactionMetadata.SetBranchID(targetBranch.ID()) {
						return nil
					}

					// iterate through the outputs of the moved transaction
					currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
						// create reference to the output
						outputID := transaction.NewOutputID(address, currentTransaction.ID())

						// load output from database
						cachedOutput := tangle.TransactionOutput(outputID)
						defer cachedOutput.Release()

						// unwrap output
						output := cachedOutput.Unwrap()
						if output == nil {
							err = fmt.Errorf("failed to load output '%s'", outputID)

							return false
						}

						// abort if the output was moved already
						if !output.SetBranchID(targetBranch.ID()) {
							return true
						}

						// schedule consumers for further checks
						consumingTransactions := make(map[transaction.ID]types.Empty)
						tangle.Consumers(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(consumer *Consumer) {
							consumingTransactions[consumer.TransactionID()] = types.Void
						})
						for transactionID := range consumingTransactions {
							transactionStack.PushBack([2]interface{}{tangle.Transaction(transactionID), tangle.TransactionMetadata(transactionID)})
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

func (tangle *Tangle) calculateBranchOfTransaction(currentTransaction *transaction.Transaction) (branch *branchmanager.CachedBranch, err error) {
	consumedBranches := make(branchmanager.BranchIds)
	if !currentTransaction.Inputs().ForEach(func(outputId transaction.OutputID) bool {
		cachedTransactionOutput := tangle.TransactionOutput(outputId)
		defer cachedTransactionOutput.Release()

		transactionOutput := cachedTransactionOutput.Unwrap()
		if transactionOutput == nil {
			err = fmt.Errorf("failed to load output '%s'", outputId)

			return false
		}

		consumedBranches[transactionOutput.BranchID()] = types.Void

		return true
	}) {
		return
	}

	branch, err = tangle.branchManager.AggregateBranches(consumedBranches.ToList()...)

	return
}

// ForEachConsumers iterates through the transactions that are consuming outputs of the given transactions
func (tangle *Tangle) ForEachConsumers(currentTransaction *transaction.Transaction, consume func(cachedTransaction *transaction.CachedTransaction, transactionMetadata *CachedTransactionMetadata, cachedAttachment *CachedAttachment)) {
	seenTransactions := make(map[transaction.ID]types.Empty)
	currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) bool {
		tangle.Consumers(transaction.NewOutputID(address, currentTransaction.ID())).Consume(func(consumer *Consumer) {
			if _, transactionSeen := seenTransactions[consumer.TransactionID()]; !transactionSeen {
				seenTransactions[consumer.TransactionID()] = types.Void

				cachedTransaction := tangle.Transaction(consumer.TransactionID())
				cachedTransactionMetadata := tangle.TransactionMetadata(consumer.TransactionID())
				for _, cachedAttachment := range tangle.Attachments(consumer.TransactionID()) {
					consume(cachedTransaction, cachedTransactionMetadata, cachedAttachment)
				}
			}
		})

		return true
	})
}
