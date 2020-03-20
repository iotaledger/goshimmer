package transferoutputmetadata

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

// CachedObject is a wrapper for the object storage, that takes care of type casting the managed objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of CachedObjects, without having to manually type cast over and over again.
type CachedObject struct {
	objectstorage.CachedObject
}

// Retain wraps the underlying method to return a new "wrapped object".
func (cachedObject *CachedObject) Retain() *CachedObject {
	return &CachedObject{cachedObject.CachedObject.Retain()}
}

// Consume wraps the underlying method to return the correctly typed objects in the callback.
func (cachedObject *CachedObject) Consume(consumer func(transferOutputMetadata *TransferOutputMetadata)) bool {
	return cachedObject.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*TransferOutputMetadata))
	})
}

// Unwrap provides a way to "Get" a type casted version of the underlying object.
func (cachedObject *CachedObject) Unwrap() *TransferOutputMetadata {
	if untypedTransaction := cachedObject.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*TransferOutputMetadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
