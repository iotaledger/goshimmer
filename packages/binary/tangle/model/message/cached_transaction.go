package message

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type CachedTransaction struct {
	objectstorage.CachedObject
}

func (cachedTransaction *CachedTransaction) Retain() *CachedTransaction {
	return &CachedTransaction{cachedTransaction.CachedObject.Retain()}
}

func (cachedTransaction *CachedTransaction) Consume(consumer func(transaction *Transaction)) bool {
	return cachedTransaction.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Transaction))
	})
}

func (cachedTransaction *CachedTransaction) Unwrap() *Transaction {
	if untypedTransaction := cachedTransaction.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Transaction); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
