package tangle

import (
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/missingtransaction"
	valuetransferpayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
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
}

func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId: storageId,

		payloadStorage:         objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayload...), valuetransferpayload.FromStorage, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayloadMetadata...), payloadmetadata.FromStorage, objectstorage.CacheTime(time.Second)),
		approverStorage:        objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferApprover...), payloadapprover.FromStorage, objectstorage.CacheTime(time.Second), objectstorage.KeysOnly(true)),
		missingPayloadStorage:  objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferMissingPayload...), missingtransaction.FromStorage, objectstorage.CacheTime(10*time.Second)),

		Events: *newEvents(),
	}

	return
}

func (tangle *Tangle) AttachPayload(payload *valuetransferpayload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

// storePayloadWorker is the worker function that stores the payload and calls the corresponding storage events.
func (tangle *Tangle) storePayloadWorker(payload *valuetransferpayload.Payload) {
	// store payload
	var cachedPayload *valuetransferpayload.CachedObject
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payload); !transactionIsNew {
		return
	} else {
		cachedPayload = &valuetransferpayload.CachedObject{CachedObject: _tmp}
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
