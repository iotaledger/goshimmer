package tangle

import (
	"container/list"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	branchManager *branchmanager.BranchManager

	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingPayloadStorage  *objectstorage.ObjectStorage

	transactionStorage         *objectstorage.ObjectStorage
	transactionMetadataStorage *objectstorage.ObjectStorage
	attachmentStorage          *objectstorage.ObjectStorage
	outputStorage              *objectstorage.ObjectStorage
	consumerStorage            *objectstorage.ObjectStorage
	valueObjectBookingStorage  *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

// New is the constructor of a Tangle and creates a new Tangle object from the given details.
func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		branchManager: branchmanager.New(badgerInstance),

		payloadStorage:         osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:        osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IDLength, payload.IDLength), objectstorage.KeysOnly(true)),

		transactionStorage:         osFactory.New(osTransaction, osTransactionFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		transactionMetadataStorage: osFactory.New(osTransactionMetadata, osTransactionMetadataFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		attachmentStorage:          osFactory.New(osAttachment, osAttachmentFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		outputStorage:              osFactory.New(osOutput, osOutputFactory, tangle.OutputKeyPartitions, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		consumerStorage:            osFactory.New(osConsumer, osConsumerFactory, tangle.ConsumerPartitionKeys, objectstorage.CacheTime(time.Second), osLeakDetectionOption),
		valueObjectBookingStorage:  osFactory.New(osValueObjectBooking, osValueObjectBookingFactory, objectstorage.CacheTime(time.Second), osLeakDetectionOption),

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
func (tangle *Tangle) TransactionMetadata(transactionID transaction.ID) *tangle.CachedTransactionMetadata {
	return &CachedTransactionMetadata{CachedObject: tangle.transactionMetadataStorage.Load(transactionID.Bytes())}
}

// TransactionOutput loads the given output from the objectstorage.
func (tangle *Tangle) TransactionOutput(outputID transaction.OutputID) *tangle.CachedOutput {
	return &CachedOutput{CachedObject: tangle.outputStorage.Load(outputID.Bytes())}
}

// Consumers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) Consumers(outputID transaction.OutputID) tangle.CachedConsumers {
	consumers := make(CachedConsumers, 0)
	tangle.consumerStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		consumers = append(consumers, &CachedConsumer{CachedObject: cachedObject})

		return true
	}, outputID.Bytes())

	return consumers
}

// Attachments retrieves the attachment of a payload from the object storage.
func (tangle *Tangle) Attachments(transactionID transaction.ID) tangle.CachedAttachments {
	attachments := make(CachedAttachments, 0)
	tangle.attachmentStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		attachments = append(attachments, &CachedAttachment{CachedObject: cachedObject})

		return true
	}, transactionID.Bytes())

	return attachments
}

// ValueObjectBooking retrieves a booking of a value object.
func (tangle *Tangle) ValueObjectBooking(id payload.ID) *tangle.CachedValueObjectBooking {
	return &CachedValueObjectBooking{CachedObject: tangle.valueObjectBookingStorage.Load(id.Bytes())}
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *payload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
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
	tangle.storePayloadWorkerPool.ShutdownGracefully()
	tangle.solidifierWorkerPool.ShutdownGracefully()
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
		tangle.valueObjectBookingStorage,
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
		tangle.valueObjectBookingStorage,
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
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyPayloadWorker(cachedPayload, cachedPayloadMetadata)
	})
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
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.Store(NewTransactionMetadata(solidPayload.Transaction().ID()))}

		// store references to the consumed outputs
		solidPayload.Transaction().Inputs().ForEach(func(outputId transaction.OutputID) bool {
			utxoDAG.consumerStorage.Store(NewConsumer(outputId, solidPayload.Transaction().ID())).Release()

			return true
		})
	} else {
		cachedTransactionMetadata = &CachedTransactionMetadata{CachedObject: utxoDAG.transactionMetadataStorage.Load(solidPayload.Transaction().ID().Bytes())}
	}

	// store a reference from the transaction to the payload that attached it or abort, if we have processed this attachment already
	attachment, stored := utxoDAG.attachmentStorage.StoreIfAbsent(NewAttachment(solidPayload.Transaction().ID(), solidPayload.ID()))
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

func (tangle *Tangle) popElementsFromSolidificationStack(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata, *CachedTransactionMetadata) {
	currentSolidificationEntry := stack.Front()
	currentCachedPayload := currentSolidificationEntry.Value.([3]interface{})[0]
	currentCachedMetadata := currentSolidificationEntry.Value.([3]interface{})[1]
	currentCachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[1]
	stack.Remove(currentSolidificationEntry)

	return currentCachedPayload.(*payload.CachedPayload), currentCachedMetadata.(*CachedPayloadMetadata), currentCachedTransactionMetadata.(*CachedTransactionMetadata)
}

// solidifyPayloadWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyPayloadWorker(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
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

			// abort if any of the retrieved models are nil
			if currentPayload == nil || currentPayloadMetadata == nil || currentTransactionMetadata == nil {
				return
			}

			currentTransaction := currentPayload.Transaction()
			fmt.Println(currentTransaction)

			if !tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) {
				return
			}

			if !currentPayloadMetadata.SetSolid(true) {
				return
			}

			// ... trigger solid event ...
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			// ... and schedule check of approvers
			tangle.ForeachApprovers(currentPayload.ID(), func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
				solidificationStack.PushBack([2]interface{}{payload, payloadMetadata})
			})
		}()
	}
}

// ForeachApprovers iterates through the approvers of a payload and calls the passed in consumer function.
func (tangle *Tangle) ForeachApprovers(payloadID payload.ID, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata)) {
	tangle.Approvers(payloadID).Consume(func(approver *PayloadApprover) {
		approvingPayloadID := approver.ApprovingPayloadID()
		approvingCachedPayload := tangle.Payload(approvingPayloadID)

		approvingCachedPayload.Consume(func(payload *payload.Payload) {
			consume(approvingCachedPayload, tangle.PayloadMetadata(approvingPayloadID))
		})
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

	return tangle.isPayloadMarkedAsSolid(payload.TrunkID()) && tangle.isPayloadMarkedAsSolid(payload.BranchID())
}

// isPayloadMarkedAsSolid returns true if the payload was marked as solid already (by setting the corresponding flags
// in its metadata.
func (tangle *Tangle) isPayloadMarkedAsSolid(payloadID payload.ID) bool {
	if payloadID == payload.GenesisID {
		return true
	}

	transactionMetadataCached := tangle.PayloadMetadata(payloadID)
	if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
		transactionMetadataCached.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(NewMissingPayload(payloadID)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*MissingPayload).ID())
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
