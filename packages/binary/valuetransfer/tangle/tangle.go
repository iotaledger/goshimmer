package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/transaction"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	storageId []byte

	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingPayloadStorage  *objectstorage.ObjectStorage
	attachmentStorage      *objectstorage.ObjectStorage

	consumerStorage                  *objectstorage.ObjectStorage
	transactionOutputMetadataStorage *objectstorage.ObjectStorage
	missingOutputStorage             *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId: storageId,

		// payload related storage
		payloadStorage:         objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayload...), payload.StorableObjectFromKey, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayloadMetadata...), PayloadMetadataFromStorageKey, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferMissingPayload...), MissingPayloadFromStorageKey, objectstorage.CacheTime(time.Second)),
		approverStorage:        objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferApprover...), PayloadApproverFromStorageKey, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),

		// transaction related storage
		transactionOutputMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.TangleApprovers...), transaction.OutputFromStorageKey, objectstorage.CacheTime(time.Second)),
		missingOutputStorage:             objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferMissingPayload...), MissingOutputFromStorageKey, objectstorage.CacheTime(time.Second)),
		consumerStorage:                  objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferConsumer...), transaction.OutputFromStorageKey, objectstorage.CacheTime(time.Second)),

		attachmentStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferAttachment...), AttachmentFromStorageKey, objectstorage.CacheTime(time.Second)),

		// transaction related storage

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

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetApprovers(transactionId payload.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, transactionId[:])

	return approvers
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
	// store payload
	var cachedPayload *payload.CachedPayload
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payloadToStore); !transactionIsNew {
		return
	} else {
		cachedPayload = &payload.CachedPayload{CachedObject: _tmp}
	}

	// store payload metadata
	payloadId := payloadToStore.Id()
	cachedMetadata := &CachedPayloadMetadata{CachedObject: tangle.payloadMetadataStorage.Store(NewPayloadMetadata(payloadId))}

	// retrieve or store TransactionMetadata
	newTransaction := false
	transactionId := cachedPayload.Unwrap().Transaction().Id()
	cachedTransactionMetadata := &CachedTransactionMetadata{CachedObject: tangle.payloadMetadataStorage.ComputeIfAbsent(transactionId.Bytes(), func(key []byte) objectstorage.StorableObject {
		newTransaction = true

		result := NewTransactionMetadata(transactionId)
		result.Persist()
		result.SetModified()

		return result
	})}

	// store trunk approver
	trunkId := payloadToStore.TrunkId()
	tangle.approverStorage.Store(NewPayloadApprover(trunkId, payloadId)).Release()

	// store branch approver
	if branchId := payloadToStore.BranchId(); branchId != trunkId {
		tangle.approverStorage.Store(NewPayloadApprover(branchId, trunkId)).Release()
	}

	// store the consumers, the first time we see a Transaction
	if newTransaction {
		payloadToStore.Transaction().Inputs().ForEach(func(outputId transaction.OutputId) bool {
			tangle.consumerStorage.Store(NewConsumer(outputId, transactionId))

			return true
		})
	}

	// store attachment
	tangle.attachmentStorage.StoreIfAbsent(NewAttachment(payloadToStore.Transaction().Id(), payloadToStore.Id()))

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadId.Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyTransactionWorker(cachedPayload, cachedMetadata, cachedTransactionMetadata)
	})
}

// solidifyTransactionWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyTransactionWorker(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata, cachedTransactionMetadata *CachedTransactionMetadata) {
	popElementsFromStack := func(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata, *CachedTransactionMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedPayload := currentSolidificationEntry.Value.([3]interface{})[0]
		currentCachedMetadata := currentSolidificationEntry.Value.([3]interface{})[1]
		currentCachedTransactionMetadata := currentSolidificationEntry.Value.([3]interface{})[2]
		stack.Remove(currentSolidificationEntry)

		return currentCachedPayload.(*payload.CachedPayload), currentCachedMetadata.(*CachedPayloadMetadata), currentCachedTransactionMetadata.(*CachedTransactionMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([3]interface{}{cachedPayload, cachedMetadata, cachedTransactionMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedPayload, currentCachedMetadata, currentCachedTransactionMetadata := popElementsFromStack(solidificationStack)

		currentPayload := currentCachedPayload.Unwrap()
		currentPayloadMetadata := currentCachedMetadata.Unwrap()
		currentTransaction := currentPayload.Transaction()
		currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
		if currentPayload == nil || currentPayloadMetadata == nil || currentTransactionMetadata == nil {
			currentCachedPayload.Release()
			currentCachedMetadata.Release()
			currentCachedTransactionMetadata.Release()

			continue
		}

		// if current transaction and payload are solid ...
		if tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) && tangle.isTransactionSolid(currentTransaction, currentTransactionMetadata) {
			payloadBecameSolid := currentPayloadMetadata.SetSolid(true)
			transactionBecameSolid := currentTransactionMetadata.SetSolid(true)

			// if payload was marked as solid the first time ...
			if payloadBecameSolid {
				tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

				tangle.GetApprovers(currentPayload.Id()).Consume(func(approver *PayloadApprover) {
					approvingPayloadId := approver.GetApprovingPayloadId()
					approvingCachedPayload := tangle.GetPayload(approvingPayloadId)

					approvingCachedPayload.Consume(func(payload *payload.Payload) {
						solidificationStack.PushBack([3]interface{}{
							approvingCachedPayload,
							tangle.GetPayloadMetadata(approvingPayloadId),
							tangle.GetTransactionMetadata(payload.Transaction().Id()),
						})
					})
				})
			}

			if transactionBecameSolid {
				tangle.Events.TransactionSolid.Trigger(currentTransaction, currentTransactionMetadata)

				currentTransaction.Inputs().ForEach(func(outputId transaction.OutputId) bool {
					return true
				})
				//tangle.GetConsumers(outputId)
			}
		}

		// release cached objects
		currentCachedPayload.Release()
		currentCachedMetadata.Release()
		currentCachedTransactionMetadata.Release()
	}
}

func (tangle *Tangle) isTransactionSolid(transaction *transaction.Transaction, metadata *TransactionMetadata) bool {
	if transaction == nil || transaction.IsDeleted() {
		return false
	}

	if metadata == nil || metadata.IsDeleted() {
		return false
	}

	if metadata.Solid() {
		return true
	}

	// iterate through all transfers and check if they are solid
	return transaction.Inputs().ForEach(tangle.isOutputMarkedAsSolid)
}

func (tangle *Tangle) GetTransferOutputMetadata(transactionOutputId transaction.OutputId) *CachedTransactionOutputMetadata {
	return &CachedTransactionOutputMetadata{CachedObject: tangle.transactionOutputMetadataStorage.Load(transactionOutputId.Bytes())}
}

func (tangle *Tangle) isOutputMarkedAsSolid(transferOutputId transaction.OutputId) (result bool) {
	objectConsumed := tangle.GetTransferOutputMetadata(transferOutputId).Consume(func(transferOutputMetadata *TransactionOutputMetadata) {
		result = transferOutputMetadata.Solid()
	})

	if !objectConsumed {
		if cachedMissingOutput, missingOutputStored := tangle.missingOutputStorage.StoreIfAbsent(NewMissingOutput(transferOutputId)); missingOutputStored {
			cachedMissingOutput.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.OutputMissing.Trigger(object.(*MissingOutput).Id())
			})
		}
	}

	return
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
