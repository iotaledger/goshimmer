package branchmanager

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

// ConflictMember represents the relationship between a Conflict and its Branches. Since a Conflict can have a
// potentially unbounded amount of conflicting Consumers, we store this as a separate k/v pair instead of a marshaled
// ist of members inside the Branch.
type ConflictMember struct {
	objectstorage.StorableObjectFlags

	conflictID ConflictID
	branchID   BranchID
}

// NewConflictMember is the constructor of the ConflictMember reference.
func NewConflictMember(conflictID ConflictID, branchID BranchID) *ConflictMember {
	return &ConflictMember{
		conflictID: conflictID,
		branchID:   branchID,
	}
}

// ConflictMemberFromBytes unmarshals a ConflictMember from a sequence of bytes.
func ConflictMemberFromBytes(bytes []byte, optionalTargetObject ...*ConflictMember) (result *ConflictMember, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseConflictMember(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictMemberFromStorageKey is a factory method that creates a new ConflictMember instance from a storage key of the
// objectstorage. It is used by the objectstorage, to create new instances of this entity.
func ConflictMemberFromStorageKey(key []byte, optionalTargetObject ...*ConflictMember) (result *ConflictMember, consumedBytes int, err error) {
	// determine the target object that will hold the unmarshaled information
	switch len(optionalTargetObject) {
	case 0:
		result = &ConflictMember{}
	case 1:
		result = optionalTargetObject[0]
	default:
		panic("too many arguments in call to ConflictMemberFromStorageKey")
	}

	// parse the properties that are stored in the key
	marshalUtil := marshalutil.New(key)
	if result.conflictID, err = ParseConflictID(marshalUtil); err != nil {
		return
	}
	if result.branchID, err = ParseBranchID(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ParseConflictMember unmarshals a ConflictMember using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseConflictMember(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*ConflictMember) (result *ConflictMember, err error) {
	parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, int, error) {
		return ConflictMemberFromStorageKey(data, optionalTargetObject...)
	})
	if parseErr != nil {
		err = parseErr

		return
	}

	result = parsedObject.(*ConflictMember)

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parsedBytes int, parseErr error) {
		parsedBytes, parseErr = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

// ConflictID returns the identifier of the Conflict that this conflictMember belongs to.
func (conflictMember *ConflictMember) ConflictID() ConflictID {
	return conflictMember.conflictID
}

// BranchID returns the identifier of the Branch that this conflictMember references.
func (conflictMember *ConflictMember) BranchID() BranchID {
	return conflictMember.branchID
}

// ObjectStorageKey returns the bytes that are used a key when storing the Branch in an objectstorage.
func (conflictMember ConflictMember) ObjectStorageKey() []byte {
	return marshalutil.New(ConflictIDLength + BranchIDLength).
		WriteBytes(conflictMember.conflictID.Bytes()).
		WriteBytes(conflictMember.branchID.Bytes()).
		Bytes()
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// ConflictMember.
func (conflictMember ConflictMember) ObjectStorageValue() []byte {
	return nil
}

// UnmarshalObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a
// marshaled Branch.
func (conflictMember ConflictMember) UnmarshalObjectStorageValue([]byte) (consumedBytes int, err error) {
	return
}

// Update is disabled but needs to be implemented to be compatible with the objectstorage.
func (conflictMember ConflictMember) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &ConflictMember{}

// CachedConflictMember is a wrapper for the generic CachedObject returned by the objectstorage that overrides the
// accessor methods, with a type-casted one.
type CachedConflictMember struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedConflictMember *CachedConflictMember) Retain() *CachedConflictMember {
	return &CachedConflictMember{cachedConflictMember.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedConflictMember *CachedConflictMember) Unwrap() *ConflictMember {
	untypedObject := cachedConflictMember.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*ConflictMember)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedConflictMember *CachedConflictMember) Consume(consumer func(conflictMember *ConflictMember), forceRelease ...bool) (consumed bool) {
	return cachedConflictMember.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ConflictMember))
	}, forceRelease...)
}

// CachedConflictMembers represents a collection of CachedConflictMembers.
type CachedConflictMembers []*CachedConflictMember

// Consume iterates over the CachedObjects, unwraps them and passes a type-casted version to the consumer (if the object
// is not empty - it exists). It automatically releases the object when the consumer finishes. It returns true, if at
// least one object was consumed.
func (cachedConflictMembers CachedConflictMembers) Consume(consumer func(conflictMember *ConflictMember)) (consumed bool) {
	for _, cachedConflictMember := range cachedConflictMembers {
		consumed = cachedConflictMember.Consume(func(output *ConflictMember) {
			consumer(output)
		}) || consumed
	}

	return
}

// Release is a utility function that allows us to release all CachedObjects in the collection.
func (cachedConflictMembers CachedConflictMembers) Release(force ...bool) {
	for _, cachedConflictMember := range cachedConflictMembers {
		cachedConflictMember.Release(force...)
	}
}
