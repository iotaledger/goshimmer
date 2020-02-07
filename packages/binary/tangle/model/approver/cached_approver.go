package approver

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type CachedApprover struct {
	objectstorage.CachedObject
}

func (cachedApprover *CachedApprover) Unwrap() *Approver {
	if untypedObject := cachedApprover.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*Approver); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

type CachedApprovers []*CachedApprover

func (cachedApprovers CachedApprovers) Consume(consumer func(approver *Approver)) (consumed bool) {
	for _, cachedApprover := range cachedApprovers {
		consumed = consumed || cachedApprover.Consume(func(object objectstorage.StorableObject) {
			consumer(object.(*Approver))
		})
	}

	return
}
