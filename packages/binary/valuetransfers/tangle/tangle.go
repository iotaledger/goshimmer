package tangle

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/iotaledger/hive.go/async"
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/storageprefix"
	valuetransferpayload "github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload"
	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/tangle/cachedpayload"
)

type Tangle struct {
	storageId []byte

	payloadStorage *objectstorage.ObjectStorage

	storePayloadWorkerPool async.WorkerPool
}

func New(badgerInstance *badger.DB, storageId []byte) (result *Tangle) {
	result = &Tangle{
		storageId: storageId,

		payloadStorage: objectstorage.New(badgerInstance, append(storageId, storageprefix.ValuetransfersPayload...), valuetransferpayload.FromStorage, objectstorage.CacheTime(1*time.Second)),
	}

	return
}

func (tangle *Tangle) AttachPayload(payload *valuetransferpayload.Payload) {
	tangle.storePayloadWorkerPool.Submit(func() { tangle.storePayloadWorker(payload) })
}

func (tangle *Tangle) storePayloadWorker(payload *valuetransferpayload.Payload) {
	// store payload
	var cachedPayload *cachedpayload.CachedPayload
	if _tmp, transactionIsNew := tangle.payloadStorage.StoreIfAbsent(payload); !transactionIsNew {
		return
	} else {
		cachedPayload = &cachedpayload.CachedPayload{CachedObject: _tmp}
	}

	fmt.Println(cachedPayload)
}
