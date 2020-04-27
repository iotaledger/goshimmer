package branchmanager

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

type ChildBranch struct {
	objectstorage.StorableObjectFlags

	parentId BranchId
	id       BranchId
}

func NewChildBranch(parentId BranchId, id BranchId) *ChildBranch {
	return &ChildBranch{
		parentId: parentId,
		id:       id,
	}
}

func ChildBranchFromBytes(bytes []byte, optionalTargetObject ...*ChildBranch) (result *ChildBranch, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseChildBranch(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ChildBranchFromStorageKey(key []byte, optionalTargetObject ...*ChildBranch) (result *ChildBranch, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &ChildBranch{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ChildBranchFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.parentId, err = ParseBranchId(marshalUtil); err != nil {
		return
	}
	if result.id, err = ParseBranchId(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseChildBranch(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*ChildBranch) (result *ChildBranch, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ChildBranchFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*ChildBranch)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

func (childBranch *ChildBranch) ParentId() BranchId {
	return childBranch.parentId
}

func (childBranch *ChildBranch) Id() BranchId {
	return childBranch.id
}

func (childBranch ChildBranch) ObjectStorageKey() []byte {
	return marshalutil.New(ConflictIdLength + BranchIdLength).
		WriteBytes(childBranch.parentId.Bytes()).
		WriteBytes(childBranch.id.Bytes()).
		Bytes()
}

func (childBranch ChildBranch) ObjectStorageValue() []byte {
	return nil
}

func (childBranch ChildBranch) UnmarshalObjectStorageValue([]byte) (err error, consumedBytes int) {
	return
}

func (childBranch ChildBranch) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &ChildBranch{}

type CachedChildBranch struct {
	objectstorage.CachedObject
}

func (cachedChildBranch *CachedChildBranch) Retain() *CachedChildBranch {
	return &CachedChildBranch{cachedChildBranch.CachedObject.Retain()}
}

func (cachedChildBranch *CachedChildBranch) Unwrap() *ChildBranch {
	if untypedObject := cachedChildBranch.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*ChildBranch); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

func (cachedChildBranch *CachedChildBranch) Consume(consumer func(childBranch *ChildBranch), forceRelease ...bool) (consumed bool) {
	return cachedChildBranch.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ChildBranch))
	}, forceRelease...)
}

type CachedChildBranches []*CachedChildBranch

func (cachedChildBranches CachedChildBranches) Consume(consumer func(childBranch *ChildBranch)) (consumed bool) {
	for _, cachedChildBranch := range cachedChildBranches {
		consumed = cachedChildBranch.Consume(func(output *ChildBranch) {
			consumer(output)
		}) || consumed
	}

	return
}

func (cachedChildBranches CachedChildBranches) Release(force ...bool) {
	for _, cachedChildBranch := range cachedChildBranches {
		cachedChildBranch.Release(force...)
	}
}
