package cachedpayload

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/valuetransfers/payload"
)

type CachedPayload struct {
	objectstorage.CachedObject
}

func (cachedPayload *CachedPayload) Retain() *CachedPayload {
	return &CachedPayload{cachedPayload.CachedObject.Retain()}
}

func (cachedPayload *CachedPayload) Consume(consumer func(payload *payload.Payload)) bool {
	return cachedPayload.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*payload.Payload))
	})
}

func (cachedPayload *CachedPayload) Unwrap() *payload.Payload {
	if untypedTransaction := cachedPayload.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*payload.Payload); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
