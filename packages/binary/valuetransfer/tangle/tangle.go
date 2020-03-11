package tangle

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/approver"
	valuetransferpayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfer/tangle/payloadmetadata"
)

type Tangle struct {
	storageId []byte

	payloadStorage         *objectstorage.ObjectStorage
	payloadMetadataStorage *objectstorage.ObjectStorage
	approverStorage        *objectstorage.ObjectStorage

	storePayloadWorkerPool async.WorkerPool
}

func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId: storageId,

		payloadStorage:         objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayload...), valuetransferpayload.FromStorage, objectstorage.CacheTime(time.Second)),
		payloadMetadataStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferPayloadMetadata...), payloadmetadata.FromStorage, objectstorage.CacheTime(time.Second)),
		approverStorage:        objectstorage.New(badgerInstance, append(storageId, storageprefix.ValueTransferApprover...), approver.FromStorage, objectstorage.CacheTime(time.Second), objectstorage.KeysOnly(true)),
	}

	return
}

func (tangle *Tangle) AttachPayload(payload *valuetransferpayload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

func (tangle *Tangle) storePayloadWorker(payload *valuetransferpayload.Payload) {
	// store payload
	var cachedPayload *valuetransferpayload.CachedObject
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payload); !transactionIsNew {
		return
	} else {
		cachedPayload = &valuetransferpayload.CachedObject{CachedObject: _tmp}
	}

	// store transaction metadata
	payloadId := payload.GetId()
	cachedPayloadMetadata := &payloadmetadata.CachedObject{CachedObject: tangle.payloadMetadataStorage.Store(payloadmetadata.New(payloadId))}

	// store trunk approver
	trunkPayloadId := payload.GetTrunkPayloadId()
	tangle.approverStorage.Store(approver.New(trunkPayloadId, payloadId)).Release()

	fmt.Println(cachedPayloadMetadata)
	fmt.Println(cachedPayload)
}
