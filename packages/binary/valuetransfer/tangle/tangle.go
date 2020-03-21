package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/missingpayload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadapprover"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadmetadata"
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

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId: storageId,

		payloadStorage:         objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayload...), valuepayload.FromStorage, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayloadMetadata...), payloadmetadata.MetadataFromStorage, objectstorage.CacheTime(time.Second)),
		approverStorage:        objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferApprover...), payloadapprover.FromStorage, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),
		missingPayloadStorage:  objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferMissingPayload...), missingpayload.FromStorage, objectstorage.CacheTime(time.Second)),

		Events: *newEvents(),
	}

	return
}

// AttachPayload adds a new payload to the value tangle.
func (tangle *Tangle) AttachPayload(payload *valuepayload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// GetPayload retrieves a payload from the object storage.
func (tangle *Tangle) GetPayload(payloadId payload.Id) *valuepayload.CachedPayload {
	return &valuepayload.CachedPayload{CachedObject: tangle.payloadStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetPayloadMetadata(payloadId payload.Id) *payloadmetadata.CachedMetadata {
	return &payloadmetadata.CachedMetadata{CachedObject: tangle.payloadMetadataStorage.Load(payloadId.Bytes())}
}

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetApprovers(transactionId payload.Id) payloadapprover.CachedObjects {
	approvers := make(payloadapprover.CachedObjects, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &payloadapprover.CachedObject{CachedObject: cachedObject})

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
func (tangle *Tangle) storePayloadWorker(payload *valuepayload.Payload) {
	// store payload
	var cachedPayload *valuepayload.CachedPayload
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payload); !transactionIsNew {
		return
	} else {
		cachedPayload = &valuepayload.CachedPayload{CachedObject: _tmp}
	}

	// store payload metadata
	payloadId := payload.GetId()
	cachedMetadata := &payloadmetadata.CachedMetadata{CachedObject: tangle.payloadMetadataStorage.Store(payloadmetadata.NewMetadata(payloadId))}

	// store trunk approver
	trunkId := payload.TrunkId()
	tangle.approverStorage.Store(payloadapprover.New(trunkId, payloadId)).Release()

	// store branch approver
	if branchId := payload.BranchId(); branchId != trunkId {
		tangle.approverStorage.Store(payloadapprover.New(branchId, trunkId)).Release()
	}

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadId.Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyTransactionWorker(cachedPayload, cachedMetadata)
	})
}

// solidifyTransactionWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyTransactionWorker(cachedPayload *valuepayload.CachedPayload, cachedMetadata *payloadmetadata.CachedMetadata) {
	popElementsFromStack := func(stack *list.List) (*valuepayload.CachedPayload, *payloadmetadata.CachedMetadata) {
		currentSolidificationEntry := stack.Front()
		currentCachedPayload := currentSolidificationEntry.Value.([2]interface{})[0]
		currentCachedMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
		stack.Remove(currentSolidificationEntry)

		return currentCachedPayload.(*valuepayload.CachedPayload), currentCachedMetadata.(*payloadmetadata.CachedMetadata)
	}

	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedPayload, cachedMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		currentCachedPayload, currentCachedMetadata := popElementsFromStack(solidificationStack)

		currentPayload := currentCachedPayload.Unwrap()
		currentMetadata := currentCachedMetadata.Unwrap()
		if currentPayload == nil || currentMetadata == nil {
			currentCachedPayload.Release()
			currentCachedMetadata.Release()

			continue
		}

		// if current transaction is solid and was not marked as solid before: mark as solid and propagate
		if tangle.isPayloadSolid(currentPayload, currentMetadata) && currentMetadata.SetSolid(true) {
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			tangle.GetApprovers(currentPayload.GetId()).Consume(func(approver *payloadapprover.PayloadApprover) {
				approverTransactionId := approver.GetApprovingPayloadId()

				solidificationStack.PushBack([2]interface{}{
					tangle.GetPayload(approverTransactionId),
					tangle.GetPayloadMetadata(approverTransactionId),
				})
			})
		}

		// release cached results
		currentCachedPayload.Release()
		currentCachedMetadata.Release()
	}
}

func (tangle *Tangle) isTransferSolid(transfer *transaction.Transaction, metadata *transaction.Metadata) bool {
	if transfer == nil || transfer.IsDeleted() {
		return false
	}

	if metadata == nil || metadata.IsDeleted() {
		return false
	}

	if metadata.Solid() {
		return true
	}

	// iterate through all transfers and check if they are solid
	return transfer.Inputs().ForEach(tangle.isTransferOutputMarkedAsSolid)
}

func (tangle *Tangle) GetTransferOutputMetadata(transferOutputId transaction.OutputId) *transaction.CachedOutputMetadata {
	return nil
}

func (tangle *Tangle) isTransferOutputMarkedAsSolid(transferOutputId transaction.OutputId) (result bool) {
	/*
		objectConsumed := tangle.GetTransferOutputMetadata(transferOutputId).Consume(func(transferOutputMetadata *transfer.OutputMetadata) {
			result = transferOutputMetadata.Solid()
		})

		if !objectConsumed {
			if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(missingpayload.NewMetadata(transferOutputId)); missingPayloadStored {
				cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
					tangle.Events.PayloadMissing.Trigger(object.(*missingpayload.MissingPayload).GetId())
				})
			}
		}
	*/

	return
}

// isPayloadSolid returns true if the given payload is solid. A payload is considered to be solid solid, if it is either
// already marked as solid or if its referenced payloads are marked as solid.
func (tangle *Tangle) isPayloadSolid(payload *valuepayload.Payload, metadata *payloadmetadata.Metadata) bool {
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
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(missingpayload.New(payloadId)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.Events.PayloadMissing.Trigger(object.(*missingpayload.MissingPayload).GetId())
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
