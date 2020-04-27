package branchmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type Branch struct {
	objectstorage.StorableObjectFlags

	id             BranchId
	parentBranches []BranchId
	conflicts      map[ConflictId]types.Empty
	preferred      bool
	liked          bool

	conflictsMutex sync.RWMutex
	preferredMutex sync.RWMutex
	likedMutex     sync.RWMutex
}

func NewBranch(id BranchId, parentBranches []BranchId, conflictingInputs []transaction.OutputId) *Branch {
	conflictingInputsMap := make(map[ConflictId]types.Empty)
	for _, conflictingInput := range conflictingInputs {
		conflictingInputsMap[conflictingInput] = types.Void
	}

	return &Branch{
		id:             id,
		parentBranches: parentBranches,
		conflicts:      conflictingInputsMap,
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
		panic("too many arguments in call to BranchFromStorageKey")
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

func (branch *Branch) Conflicts() (conflicts map[ConflictId]types.Empty) {
	branch.conflictsMutex.RLock()
	defer branch.conflictsMutex.RUnlock()

	conflicts = make(map[ConflictId]types.Empty, len(branch.conflicts))
	for conflict := range branch.conflicts {
		conflicts[conflict] = types.Void
	}

	return
}

func (branch *Branch) AddConflict(conflict ConflictId) (added bool) {
	branch.conflictsMutex.RLock()
	if _, exists := branch.conflicts[conflict]; exists {
		branch.conflictsMutex.RUnlock()

		return
	}

	branch.conflictsMutex.RUnlock()
	branch.conflictsMutex.Lock()
	defer branch.conflictsMutex.Unlock()

	if _, exists := branch.conflicts[conflict]; exists {
		return
	}

	branch.conflicts[conflict] = types.Void
	added = true

	return
}

func (branch *Branch) Preferred() bool {
	branch.preferredMutex.RLock()
	defer branch.preferredMutex.RUnlock()

	return branch.preferred
}

func (branch *Branch) SetPreferred(preferred bool) (modified bool) {
	branch.preferredMutex.RLock()
	if branch.preferred == preferred {
		branch.preferredMutex.RUnlock()

		return
	}

	branch.preferredMutex.RUnlock()
	branch.preferredMutex.Lock()
	defer branch.preferredMutex.Lock()

	if branch.preferred == preferred {
		return
	}

	branch.preferred = preferred
	modified = true

	return branch.preferred
}

func (branch *Branch) Liked() bool {
	branch.likedMutex.RLock()
	defer branch.likedMutex.RUnlock()

	return branch.liked
}

func (branch *Branch) SetLiked(liked bool) (modified bool) {
	branch.likedMutex.RLock()
	if branch.liked == liked {
		branch.likedMutex.RUnlock()

		return
	}

	branch.likedMutex.RUnlock()
	branch.likedMutex.Lock()
	defer branch.likedMutex.Lock()

	if branch.liked == liked {
		return
	}

	branch.liked = liked
	modified = true

	return branch.liked
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
	branch.preferredMutex.RLock()
	branch.likedMutex.RLock()
	defer branch.preferredMutex.RUnlock()
	defer branch.likedMutex.RUnlock()

	parentBranches := branch.ParentBranches()
	parentBranchCount := len(parentBranches)

	marshalUtil := marshalutil.New(2*marshalutil.BOOL_SIZE + marshalutil.UINT32_SIZE + parentBranchCount*BranchIdLength)
	marshalUtil.WriteBool(branch.preferred)
	marshalUtil.WriteBool(branch.liked)
	marshalUtil.WriteUint32(uint32(parentBranchCount))
	for _, branchId := range parentBranches {
		marshalUtil.WriteBytes(branchId.Bytes())
	}

	return marshalUtil.Bytes()
}

func (branch *Branch) UnmarshalObjectStorageValue(valueBytes []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(valueBytes)
	branch.preferred, err = marshalUtil.ReadBool()
	if err != nil {
		return
	}
	branch.liked, err = marshalUtil.ReadBool()
	if err != nil {
		return
	}
	parentBranchCount, err := marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	branch.parentBranches = make([]BranchId, parentBranchCount)
	for i := uint32(0); i < parentBranchCount; i++ {
		branch.parentBranches[i], err = ParseBranchId(marshalUtil)
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

func (cachedBranches *CachedBranch) Retain() *CachedBranch {
	return &CachedBranch{cachedBranches.CachedObject.Retain()}
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
