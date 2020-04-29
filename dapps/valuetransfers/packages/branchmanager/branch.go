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

	id             BranchID
	parentBranches []BranchID
	conflicts      map[ConflictID]types.Empty
	preferred      bool
	liked          bool

	conflictsMutex sync.RWMutex
	preferredMutex sync.RWMutex
	likedMutex     sync.RWMutex
}

// NewBranch is the constructor of a Branch and creates a new Branch object from the given details.
func NewBranch(id BranchID, parentBranches []BranchID, conflictingInputs []transaction.OutputID) *Branch {
	conflictingInputsMap := make(map[ConflictID]types.Empty)
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
	result.id, err = ParseBranchID(marshalUtil)
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
func (branch *Branch) ID() BranchID {
	return branch.id
}

// ParentBranches returns the identifiers of the parents of this Branch.
func (branch *Branch) ParentBranches() []BranchID {
	return branch.parentBranches
}

// IsAggregated returns true if the branch is not a conflict-branch, but was created by merging multiple other branches.
func (branch *Branch) IsAggregated() bool {
	return len(branch.parentBranches) > 1
}

// Conflicts retrieves the Conflicts that a Branch is part of.
func (branch *Branch) Conflicts() (conflicts map[ConflictID]types.Empty) {
	branch.conflictsMutex.RLock()
	defer branch.conflictsMutex.RUnlock()

	conflicts = make(map[ConflictID]types.Empty, len(branch.conflicts))
	for conflict := range branch.conflicts {
		conflicts[conflict] = types.Void
	}

	return
}

// AddConflict registers the membership of this Branch in a given
func (branch *Branch) AddConflict(conflict ConflictID) (added bool) {
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

// ObjectStorageKey returns the bytes that are used as a key when storing the Branch in an objectstorage.
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

	marshalUtil := marshalutil.New(2*marshalutil.BOOL_SIZE + marshalutil.UINT32_SIZE + parentBranchCount*BranchIDLength)
	marshalUtil.WriteBool(branch.preferred)
	marshalUtil.WriteBool(branch.liked)
	marshalUtil.WriteUint32(uint32(parentBranchCount))
	for _, branchID := range parentBranches {
		marshalUtil.WriteBytes(branchID.Bytes())
	}

	return marshalUtil.Bytes()
}

// UnmarshalObjectStorageValue unmarshals the bytes that are stored in the value of the objectstorage.
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
	branch.parentBranches = make([]BranchID, parentBranchCount)
	for i := uint32(0); i < parentBranchCount; i++ {
		branch.parentBranches[i], err = ParseBranchID(marshalUtil)
		if err != nil {
			return
		}
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// CachedBranch is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedBranch struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedBranch *CachedBranch) Retain() *CachedBranch {
	return &CachedBranch{cachedBranch.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedBranch *CachedBranch) Unwrap() *Branch {
	untypedObject := cachedBranch.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Branch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedBranch *CachedBranch) Consume(consumer func(branch *Branch), forceRelease ...bool) (consumed bool) {
	return cachedBranch.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Branch))
	}, forceRelease...)
}

// CachedBranches represents a collection of CachedBranches.
type CachedBranches map[BranchID]*CachedBranch

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedBranches CachedBranches) Consume(consumer func(branch *Branch)) (consumed bool) {
	for _, cachedBranch := range cachedBranches {
		consumed = cachedBranch.Consume(func(output *Branch) {
			consumer(output)
		}) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (cachedBranches CachedBranches) Release(force ...bool) {
	for _, cachedBranch := range cachedBranches {
		cachedBranch.Release(force...)
	}
}
