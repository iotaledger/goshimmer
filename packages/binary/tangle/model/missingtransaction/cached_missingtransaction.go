package missingtransaction

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type CachedMissingTransaction struct {
	objectstorage.CachedObject
}

func (cachedObject *CachedMissingTransaction) Unwrap() *MissingTransaction {
	if untypedObject := cachedObject.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*MissingTransaction); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}
