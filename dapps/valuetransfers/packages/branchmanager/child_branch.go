package branchmanager

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// ChildBranch represents the relationship between a Branch and its children. Since a Branch can have a potentially
// unbounded amount of child Branches, we store this as a separate k/v pair instead of a marshaled list of children
// inside the Branch.
type ChildBranch struct {
	objectstorage.StorableObjectFlags

	parentID BranchID
	childID  BranchID
}

// NewChildBranch is the constructor of the ChildBranch reference.
func NewChildBranch(parentID BranchID, childID BranchID) *ChildBranch {
	return &ChildBranch{
		parentID: parentID,
		childID:  childID,
	}
}

// ChildBranchFromBytes unmarshals a ChildBranch from a sequence of bytes.
func ChildBranchFromBytes(bytes []byte, optionalTargetObject ...*ChildBranch) (result *ChildBranch, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseChildBranch(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ChildBranchFromStorageKey is a factory method that creates a new ChildBranch instance from a storage key of the
// objectstorage. It is used by the objectstorage, to create new instances of this entity.
func ChildBranchFromStorageKey(key []byte, optionalTargetObject ...*ChildBranch) (result *ChildBranch, consumedBytes int, err error) {
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
	if result.parentID, err = ParseBranchID(marshalUtil); err != nil {
		return
	}
	if result.childID, err = ParseBranchID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseChildBranch unmarshals a ChildBranch using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseChildBranch(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*ChildBranch) (result *ChildBranch, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ChildBranchFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*ChildBranch)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// ParentID returns the ID of the Branch that plays the role of the parent in this relationship.
func (childBranch *ChildBranch) ParentID() BranchID {
	return childBranch.parentID
}

// ChildID returns the ID of the Branch that plays the role of the child in this relationship.
func (childBranch *ChildBranch) ChildID() BranchID {
	return childBranch.childID
}

// ObjectStorageKey returns the bytes that are used a key when storing the Branch in an objectstorage.
func (childBranch ChildBranch) ObjectStorageKey() []byte {
	return marshalutil.New(ConflictIDLength + BranchIDLength).
		WriteBytes(childBranch.parentID.Bytes()).
		WriteBytes(childBranch.childID.Bytes()).
		Bytes()
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// ChildBranch.
func (childBranch ChildBranch) ObjectStorageValue() []byte {
	return nil
}

// UnmarshalObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a
// marshaled Branch.
func (childBranch ChildBranch) UnmarshalObjectStorageValue([]byte) (consumedBytes int, err error) {
	return
}

// Update is disabled but needs to be implemented to be compatible with the objectstorage.
func (childBranch ChildBranch) Update(objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &ChildBranch{}

// CachedChildBranch is a wrapper for the generic CachedObject returned by the objectstorage that overrides the
// accessor methods, with a type-casted one.
type CachedChildBranch struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedChildBranch *CachedChildBranch) Retain() *CachedChildBranch {
	return &CachedChildBranch{cachedChildBranch.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedChildBranch *CachedChildBranch) Unwrap() *ChildBranch {
	untypedObject := cachedChildBranch.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*ChildBranch)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedChildBranch *CachedChildBranch) Consume(consumer func(childBranch *ChildBranch), forceRelease ...bool) (consumed bool) {
	return cachedChildBranch.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ChildBranch))
	}, forceRelease...)
}

// CachedChildBranches represents a collection of CachedChildBranches.
type CachedChildBranches []*CachedChildBranch

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedChildBranches CachedChildBranches) Consume(consumer func(childBranch *ChildBranch)) (consumed bool) {
	for _, cachedChildBranch := range cachedChildBranches {
		consumed = cachedChildBranch.Consume(func(output *ChildBranch) {
			consumer(output)
		}) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (cachedChildBranches CachedChildBranches) Release(force ...bool) {
	for _, cachedChildBranch := range cachedChildBranches {
		cachedChildBranch.Release(force...)
	}
}
