package branchmanager

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

type Branch struct {
	objectstorage.StorableObjectFlags

	id             BranchId
	parentBranches []BranchId
}

func NewBranch(id BranchId, parentBranches []BranchId) *Branch {
	return &Branch{
		id:             id,
		parentBranches: parentBranches,
	}
}

func BranchFromStorageKey(key []byte, optionalTargetObject ...*Branch) (result *Branch, err error, consumedBytes int) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &Branch{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to TransactionMetadataFromStorageKey")
	}

	// parse information
	marshalUtil := marshalutil.New(key)
	result.id, err = ParseBranchId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func BranchFromBytes(bytes []byte, optionalTargetObject ...*Branch) (result *Branch, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseBranch(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseBranch(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Branch) (result *Branch, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return BranchFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*Branch)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
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

func (branch *Branch) Bytes() []byte {
	return marshalutil.New().
		WriteBytes(branch.ObjectStorageKey()).
		WriteBytes(branch.ObjectStorageValue()).
		Bytes()
}

func (branch *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", branch.Id()),
	)
}

func (branch *Branch) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - please use the setters")
}

func (branch *Branch) ObjectStorageKey() []byte {
	return branch.id.Bytes()
}

func (branch *Branch) ObjectStorageValue() []byte {
	parentBranches := branch.ParentBranches()
	parentBranchCount := len(parentBranches)

	marshalUtil := marshalutil.New(marshalutil.UINT32_SIZE + parentBranchCount*BranchIdLength)
	marshalUtil.WriteUint32(uint32(parentBranchCount))
	for _, branchId := range parentBranches {
		marshalUtil.WriteBytes(branchId.Bytes())
	}

	return marshalUtil.Bytes()
}

func (branch *Branch) UnmarshalObjectStorageValue(valueBytes []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(valueBytes)
	parentBranchCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	parentBranches := make([]BranchId, parentBranchCount)
	for i := uint32(0); i < parentBranchCount; i++ {
		parentBranches[i], err = ParseBranchId(marshalUtil)
		if err != nil {
			return
		}
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
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
