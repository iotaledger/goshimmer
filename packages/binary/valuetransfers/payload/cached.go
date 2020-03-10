package payload

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type Cached struct {
	objectstorage.CachedObject
}

func (cachedPayload *Cached) Retain() *Cached {
	return &Cached{cachedPayload.CachedObject.Retain()}
}

func (cachedPayload *Cached) Consume(consumer func(payload *Payload)) bool {
	return cachedPayload.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Payload))
	})
}

func (cachedPayload *Cached) Unwrap() *Payload {
	if untypedTransaction := cachedPayload.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Payload); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
