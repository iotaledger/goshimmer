package transfer

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

// CachedMetadata is a wrapper for the object storage, that takes care of type casting the Metadata objects.
// Since go does not have generics (yet), the object storage works based on the generic "interface{}" type, which means
// that we have to regularly type cast the returned objects, to match the expected type. To reduce the burden of
// manually managing these type, we create a wrapper that does this for us. This way, we can consistently handle the
// specialized types of Metadata, without having to manually type cast over and over again.
type CachedMetadata struct {
	objectstorage.CachedObject
}

// Retain overrides the underlying method to return a new CachedMetadata instead of a generic CachedObject.
func (cachedMetadata *CachedMetadata) Retain() *CachedMetadata {
	return &CachedMetadata{cachedMetadata.CachedObject.Retain()}
}

// Consume  overrides the underlying method to use a CachedMetadata object instead of a generic CachedObject in the
// consumer).
func (cachedMetadata *CachedMetadata) Consume(consumer func(metadata *Metadata)) bool {
	return cachedMetadata.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Metadata))
	})
}

// Unwrap provides a way to retrieve a type casted version of the underlying object.
func (cachedMetadata *CachedMetadata) Unwrap() *Metadata {
	if untypedTransaction := cachedMetadata.Get(); untypedTransaction == nil {
		return nil
	} else {
		if typeCastedTransaction := untypedTransaction.(*Metadata); typeCastedTransaction == nil || typeCastedTransaction.IsDeleted() {
			return nil
		} else {
			return typeCastedTransaction
		}
	}
}
