package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/address"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/balance"
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

	outputStorage        *objectstorage.ObjectStorage
	consumerStorage      *objectstorage.ObjectStorage
	missingOutputStorage *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		// payload related storage
		payloadStorage:         osFactory.New(PrefixPayload, payload.StorableObjectFromKey, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(PrefixPayloadMetadata, PayloadMetadataFromStorageKey, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  osFactory.New(PrefixMissingPayload, MissingPayloadFromStorageKey, objectstorage.CacheTime(time.Second)),
		approverStorage:        osFactory.New(PrefixApprover, PayloadApproverFromStorageKey, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),

		// transaction related storages
		attachmentStorage:    osFactory.New(PrefixAttachment, attachmentFromStorageKey, objectstorage.CacheTime(time.Second)),
		outputStorage:        osFactory.New(PrefixOutput, outputFromStorageKey, transaction.OutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		missingOutputStorage: osFactory.New(PrefixMissingOutput, missingOutputFromStorageKey, MissingOutputKeyPartitions, objectstorage.CacheTime(time.Second)),
		consumerStorage:      osFactory.New(PrefixConsumer, consumerFromStorageKey, ConsumerPartitionKeys, objectstorage.CacheTime(time.Second)),

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

func (tangle *Tangle) GetTransactionOutput(outputId transaction.OutputId) *transaction.CachedOutput {
	return &transaction.CachedOutput{CachedObject: tangle.outputStorage.Load(outputId.Bytes())}
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

// GetApprovers retrieves the approvers of a payload from the object storage.
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

	if transactionStored {
		tx.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) {
			tangle.outputStorage.Store(transaction.NewOutput(address, tx.Id(), balances))
		})
	}

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
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedPayload, currentCachedMetadata, currentCachedTransactionMetadata := popElementsFromStack(solidificationStack)
			defer currentCachedPayload.Release()
			defer currentCachedMetadata.Release()
			defer currentCachedTransactionMetadata.Release()

			// unwrap cached objects
			currentPayload := currentCachedPayload.Unwrap()
			currentPayloadMetadata := currentCachedMetadata.Unwrap()
			currentTransactionMetadata := currentCachedTransactionMetadata.Unwrap()
			currentTransaction := currentPayload.Transaction()

			// abort if any of the retrieved models is nil
			if currentPayload == nil || currentPayloadMetadata == nil || currentTransactionMetadata == nil {
				return
			}

			// abort if the entities are not solid
			if !tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) || !tangle.isTransactionSolid(currentTransaction, currentTransactionMetadata) {
				return
			}

			// abort if the payload was marked as solid already (if a payload is solid already then the tx is also solid)
			payloadBecameSolid := currentPayloadMetadata.SetSolid(true)
			if !payloadBecameSolid {
				return
			}

			// set the transaction related entities to be solid
			transactionBecameSolid := currentTransactionMetadata.SetSolid(true)
			if transactionBecameSolid {
				currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) {
					tangle.GetTransactionOutput(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(output *transaction.Output) {
						output.SetSolid(true)
					})
				})
			}

			// ... trigger solid event ...
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			// ... and schedule check of approvers
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

			if !transactionBecameSolid {
				return
			}

			tangle.Events.TransactionSolid.Trigger(currentTransaction, currentTransactionMetadata)

			seenTransactions := make(map[transaction.Id]types.Empty)
			currentTransaction.Outputs().ForEach(func(address address.Address, balances []*balance.Balance) {
				tangle.GetTransactionOutput(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(output *transaction.Output) {
					// trigger events
				})

				tangle.GetConsumers(transaction.NewOutputId(address, currentTransaction.Id())).Consume(func(consumer *Consumer) {
					// keep track of the processed transactions (the same transaction can consume multiple outputs)
					if _, transactionSeen := seenTransactions[consumer.TransactionId()]; transactionSeen {
						seenTransactions[consumer.TransactionId()] = types.Void

						transactionMetadata := tangle.GetTransactionMetadata(consumer.TransactionId())

						// retrieve all the payloads that attached the transaction
						tangle.GetAttachments(consumer.TransactionId()).Consume(func(attachment *Attachment) {
							solidificationStack.PushBack([3]interface{}{
								tangle.GetPayload(attachment.PayloadId()),
								tangle.GetPayloadMetadata(attachment.PayloadId()),
								transactionMetadata,
							})
						})
					}
				})
			})
		}()
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

func (tangle *Tangle) isOutputMarkedAsSolid(transactionOutputId transaction.OutputId) (result bool) {
	outputExists := tangle.GetTransactionOutput(transactionOutputId).Consume(func(output *transaction.Output) {
		result = output.Solid()
	})

	if !outputExists {
		if cachedMissingOutput, missingOutputStored := tangle.missingOutputStorage.StoreIfAbsent(NewMissingOutput(transactionOutputId)); missingOutputStored {
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

func outputFromStorageKey(key []byte) (objectstorage.StorableObject, error, int) {
	return transaction.OutputFromStorageKey(key)
}

func missingOutputFromStorageKey(key []byte) (objectstorage.StorableObject, error, int) {
	return MissingOutputFromStorageKey(key)
}

func consumerFromStorageKey(key []byte) (objectstorage.StorableObject, error, int) {
	return ConsumerFromStorageKey(key)
}

func attachmentFromStorageKey(key []byte) (objectstorage.StorableObject, error, int) {
	return AttachmentFromStorageKey(key)
}
