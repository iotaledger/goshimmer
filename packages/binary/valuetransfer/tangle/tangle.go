package tangle

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	valuepayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	payloadid "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload/id"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/missingpayload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadapprover"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadmetadata"
)

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
		payloadMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayloadMetadata...), payloadmetadata.FromStorage, objectstorage.CacheTime(time.Second)),
		approverStorage:        objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferApprover...), payloadapprover.FromStorage, objectstorage.CacheTime(time.Second), objectstorage.KeysOnly(true)),
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
func (tangle *Tangle) GetPayload(payloadId payloadid.Id) *valuepayload.CachedObject {
	return &valuepayload.CachedObject{CachedObject: tangle.payloadStorage.Load(payloadId.Bytes())}
}

// GetPayloadMetadata retrieves the metadata of a value payload from the object storage.
func (tangle *Tangle) GetPayloadMetadata(payloadId payloadid.Id) *payloadmetadata.CachedObject {
	return &payloadmetadata.CachedObject{CachedObject: tangle.payloadMetadataStorage.Load(payloadId.Bytes())}
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

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payload *valuepayload.Payload) {
	// store payload
	var cachedPayload *valuepayload.CachedObject
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payload); !transactionIsNew {
		return
	} else {
		cachedPayload = &valuepayload.CachedObject{CachedObject: _tmp}
	}

	// store payload metadata
	payloadId := payload.GetId()
	cachedMetadata := &payloadmetadata.CachedObject{CachedObject: tangle.payloadMetadataStorage.Store(payloadmetadata.New(payloadId))}

	// store trunk approver
	trunkId := payload.GetTrunkId()
	tangle.approverStorage.Store(payloadapprover.New(trunkId, payloadId)).Release()

	// store branch approver
	if branchId := payload.GetBranchId(); branchId != trunkId {
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

func (tangle *Tangle) solidifyTransactionWorker(cachedPayload *valuepayload.CachedObject, cachedMetadata *payloadmetadata.CachedObject) {

}

func (tangle *Tangle) isTransactionMarkedAsSolid(payloadId payloadid.Id) bool {
	if payloadId == payloadid.Genesis {
		return true
	}

	transactionMetadataCached := tangle.GetPayloadMetadata(payloadId)
	if transactionMetadata := transactionMetadataCached.Unwrap(); transactionMetadata == nil {
		transactionMetadataCached.Release()

		// if transaction is missing and was not reported as missing, yet
		if cachedMissingPayload, missingPayloadStored := tangle.missingPayloadStorage.StoreIfAbsent(missingpayload.New(payloadId)); missingPayloadStored {
			cachedMissingPayload.Consume(func(object objectstorage.StorableObject) {
				tangle.monitorMissingTransactionWorker(object.(*missingpayload.MissingPayload).GetId())
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

func (tangle *Tangle) monitorMissingTransactionWorker(id payloadid.Id) {
	// DO SOMETHING
}
