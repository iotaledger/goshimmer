package tangle

import (
	"container/list"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
)

// Tangle represents the value tangle that consists out of value payloads.
// It is an independent ontology, that lives inside the tangle.
type Tangle struct {
	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage
	missingPayloadStorage  *objectstorage.ObjectStorage

	Events Events

	storePayloadWorkerPool async.WorkerPool
	solidifierWorkerPool   async.WorkerPool
	cleanupWorkerPool      async.WorkerPool
}

func New(badgerInstance *badger.DB) (result *Tangle) {
	osFactory := objectstorage.NewFactory(badgerInstance, storageprefix.ValueTransfers)

	result = &Tangle{
		payloadStorage:         osFactory.New(osPayload, osPayloadFactory, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: osFactory.New(osPayloadMetadata, osPayloadMetadataFactory, objectstorage.CacheTime(time.Second)),
		missingPayloadStorage:  osFactory.New(osMissingPayload, osMissingPayloadFactory, objectstorage.CacheTime(time.Second)),
		approverStorage:        osFactory.New(osApprover, osPayloadApproverFactory, objectstorage.CacheTime(time.Second), objectstorage.PartitionKey(payload.IdLength, payload.IdLength), objectstorage.KeysOnly(true)),

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

// GetApprovers retrieves the approvers of a payload from the object storage.
func (tangle *Tangle) GetApprovers(payloadId payload.Id) CachedApprovers {
	approvers := make(CachedApprovers, 0)
	tangle.approverStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		approvers = append(approvers, &CachedPayloadApprover{CachedObject: cachedObject})

		return true
	}, payloadId.Bytes())

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
	// store the payload and transaction models
	cachedPayload, cachedPayloadMetadata, payloadStored := tangle.storePayload(payloadToStore)
	if !payloadStored {
		// abort if we have seen the payload already
		return
	}

	// store the references between the different entities (we do this after the actual entities were stored, so that
	// all the metadata models exist in the database as soon as the entities are reachable by walks).
	tangle.storePayloadReferences(payloadToStore)

	// trigger events
	if tangle.missingPayloadStorage.DeleteIfPresent(payloadToStore.Id().Bytes()) {
		tangle.Events.MissingPayloadReceived.Trigger(cachedPayload, cachedPayloadMetadata)
	}
	tangle.Events.PayloadAttached.Trigger(cachedPayload, cachedPayloadMetadata)

	// check solidity
	tangle.solidifierWorkerPool.Submit(func() {
		tangle.solidifyPayloadWorker(cachedPayload, cachedPayloadMetadata)
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

func (tangle *Tangle) storePayloadReferences(payload *payload.Payload) {
	// store trunk approver
	trunkId := payload.TrunkId()
	tangle.approverStorage.Store(NewPayloadApprover(trunkId, payload.Id())).Release()

	// store branch approver
	if branchId := payload.BranchId(); branchId != trunkId {
		tangle.approverStorage.Store(NewPayloadApprover(branchId, trunkId)).Release()
	}
}

func (tangle *Tangle) popElementsFromSolidificationStack(stack *list.List) (*payload.CachedPayload, *CachedPayloadMetadata) {
	currentSolidificationEntry := stack.Front()
	currentCachedPayload := currentSolidificationEntry.Value.([2]interface{})[0]
	currentCachedMetadata := currentSolidificationEntry.Value.([2]interface{})[1]
	stack.Remove(currentSolidificationEntry)

	return currentCachedPayload.(*payload.CachedPayload), currentCachedMetadata.(*CachedPayloadMetadata)
}

// solidifyPayloadWorker is the worker function that solidifies the payloads (recursively from past to present).
func (tangle *Tangle) solidifyPayloadWorker(cachedPayload *payload.CachedPayload, cachedMetadata *CachedPayloadMetadata) {
	// initialize the stack
	solidificationStack := list.New()
	solidificationStack.PushBack([2]interface{}{cachedPayload, cachedMetadata})

	// process payloads that are supposed to be checked for solidity recursively
	for solidificationStack.Len() > 0 {
		// execute logic inside a func, so we can use defer to release the objects
		func() {
			// retrieve cached objects
			currentCachedPayload, currentCachedMetadata := tangle.popElementsFromSolidificationStack(solidificationStack)
			defer currentCachedPayload.Release()
			defer currentCachedMetadata.Release()

			// unwrap cached objects
			currentPayload := currentCachedPayload.Unwrap()
			currentPayloadMetadata := currentCachedMetadata.Unwrap()

			// abort if any of the retrieved models is nil or payload is not solid or it was set as solid already
			if currentPayload == nil || currentPayloadMetadata == nil || !tangle.isPayloadSolid(currentPayload, currentPayloadMetadata) || !currentPayloadMetadata.SetSolid(true) {
				return
			}

			// ... trigger solid event ...
			tangle.Events.PayloadSolid.Trigger(currentCachedPayload, currentCachedMetadata)

			// ... and schedule check of approvers
			tangle.ForeachApprovers(currentPayload.Id(), func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata) {
				solidificationStack.PushBack([2]interface{}{payload, payloadMetadata})
			})
		}()
	}
}

func (tangle *Tangle) ForeachApprovers(payloadId payload.Id, consume func(payload *payload.CachedPayload, payloadMetadata *CachedPayloadMetadata)) {
	tangle.GetApprovers(payloadId).Consume(func(approver *PayloadApprover) {
		approvingPayloadId := approver.GetApprovingPayloadId()
		approvingCachedPayload := tangle.GetPayload(approvingPayloadId)

		approvingCachedPayload.Consume(func(payload *payload.Payload) {
			consume(approvingCachedPayload, tangle.GetPayloadMetadata(approvingPayloadId))
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
