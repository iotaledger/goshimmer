package branchmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

// Branch represents a part of the tangle, that shares the same perception of the ledger state. Every conflicting
// transaction formw a Branch, that contains all transactions that are spending Outputs of the conflicting transactions.
// Branches can also be created by merging two other Branches, which creates an aggregated Branch.
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

// NewBranch is the constructor of a branch and creates a new Branch object from the given details.
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

// BranchFromStorageKey is a factory method that creates a new Branch instance from a storage key of the objectstorage.
// It is used by the objectstorage, to create new instances of this entity.
func BranchFromStorageKey(key []byte, optionalTargetObject ...*Branch) (result *Branch, consumedBytes int, err error) {
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

// BranchFromBytes unmarshals a Branch from a sequence of bytes.
func BranchFromBytes(bytes []byte, optionalTargetObject ...*Branch) (result *Branch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseBranch(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseBranch unmarshals a Branch using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseBranch(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*Branch) (result *Branch, err error) {
	parsedObject, err := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return BranchFromStorageKey(data, optionalTargetObject...)
	})
	if err != nil {
		return
	}

	result = parsedObject.(*Branch)
	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// ID returns the identifier of the Branch (usually the transaction.ID that created the branch - unless its an
// aggregated Branch).
func (branch *Branch) ID() BranchId {
	return branch.id
}

// ParentBranches returns the identifiers of the parents of this Branch.
func (branch *Branch) ParentBranches() []BranchId {
	return branch.parentBranches
}

// IsAggregated returns true if the branch is not a conflict-branch, but was created by merging multiple other branches.
func (branch *Branch) IsAggregated() bool {
	return len(branch.parentBranches) > 1
}

// Conflicts retrieves the Conflicts that a Branch is part of.
func (branch *Branch) Conflicts() (conflicts map[ConflictId]types.Empty) {
	branch.conflictsMutex.RLock()
	defer branch.conflictsMutex.RUnlock()

	conflicts = make(map[ConflictId]types.Empty, len(branch.conflicts))
	for conflict := range branch.conflicts {
		conflicts[conflict] = types.Void
	}

	return
}

// AddConflict registers the membership of this Branch in a given
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

// Preferred returns true, if the branch is the favored one among the branches in the same conflict sets.
func (branch *Branch) Preferred() bool {
	branch.preferredMutex.RLock()
	defer branch.preferredMutex.RUnlock()

	return branch.preferred
}

// SetPreferred is the setter for the preferred flag. It returns true if the value of the flag has been updated.
// A branch is preferred if it represents the "liked" part of the tangle in it corresponding Branch.
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

// Liked returns if the branch is liked (it is preferred and all of its parents are liked).
func (branch *Branch) Liked() bool {
	branch.likedMutex.RLock()
	defer branch.likedMutex.RUnlock()

	return branch.liked
}

// SetLiked modifies the liked flag of this branch. It returns true, if the current value has been modified.
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

// Bytes returns a marshaled version of this Branch.
func (branch *Branch) Bytes() []byte {
	return marshalutil.New().
		WriteBytes(branch.ObjectStorageKey()).
		WriteBytes(branch.ObjectStorageValue()).
		Bytes()
}

// String returns a human readable version of this Branch (for debug purposes).
func (branch *Branch) String() string {
	return stringify.Struct("Branch",
		stringify.StructField("id", branch.ID()),
	)
}

// Update is disabled but needs to be implemented to be compatible with the objectstorage.
func (branch *Branch) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - please use the setters")
}

// ObjectStorageKey returns the bytes that are used a key when storing the Branch in an objectstorage.
func (branch *Branch) ObjectStorageKey() []byte {
	return branch.id.Bytes()
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// Branch.
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
	for _, branchID := range parentBranches {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return marshalUtil.Bytes()
}

func (branch *Branch) UnmarshalObjectStorageValue(valueBytes []byte) (consumedBytes int, err error) {
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
