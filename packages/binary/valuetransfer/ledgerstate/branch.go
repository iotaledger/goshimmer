package ledgerstate

import (
	"github.com/iotaledger/hive.go/objectstorage"
)

type Branch struct {
	objectstorage.StorableObjectFlags

	id             BranchId
	parentBranches []BranchId
}

func (branch *Branch) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

func (branch *Branch) ObjectStorageKey() []byte {
	panic("implement me")
}

func (branch *Branch) ObjectStorageValue() []byte {
	panic("implement me")
}

func (branch *Branch) UnmarshalObjectStorageValue(valueBytes []byte) (err error, consumedBytes int) {
	panic("implement me")
}

func NewBranch(id BranchId, parentBranches []BranchId) *Branch {
	return nil
}

func (branch *Branch) Id() BranchId {
	return branch.id
}

func (branch *Branch) ParentBranches() []BranchId {
	return branch.parentBranches
}

func (branch *Branch) IsAggregated() bool {
	return len(branch.parentBranches) > 1
}

type CachedBranch struct {
	objectstorage.CachedObject
}

func (cachedBranches *CachedBranch) Unwrap() *Branch {
	if untypedObject := cachedBranches.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*Branch); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

func (cachedBranches *CachedBranch) Consume(consumer func(branch *Branch), forceRelease ...bool) (consumed bool) {
	return cachedBranches.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Branch))
	}, forceRelease...)
}

type CachedBranches map[BranchId]*CachedBranch

func (cachedBranches CachedBranches) Consume(consumer func(branch *Branch)) (consumed bool) {
	for _, cachedBranch := range cachedBranches {
		consumed = cachedBranch.Consume(func(output *Branch) {
			consumer(output)
		}) || consumed
	}

	return
}

func (cachedBranches CachedBranches) Release(force ...bool) {
	for _, cachedBranch := range cachedBranches {
		cachedBranch.Release(force...)
	}
}
