package branchmanager

import (
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
)

type ConflictMember struct {
	objectstorage.StorableObjectFlags

	conflictId ConflictId
	branchId   BranchId
}

func NewConflictMember(conflictId ConflictId, branchId BranchId) *ConflictMember {
	return &ConflictMember{
		conflictId: conflictId,
		branchId:   branchId,
	}
}

func ConflictMemberFromBytes(bytes []byte, optionalTargetObject ...*ConflictMember) (result *ConflictMember, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseConflictMember(marshalUtil, optionalTargetObject...)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ConflictMemberFromStorageKey(key []byte, optionalTargetObject ...*ConflictMember) (result *ConflictMember, err error, consumedBytes int) {
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
	if result.conflictId, err = ParseConflictId(marshalUtil); err != nil {
		return
	}
	if result.branchId, err = ParseBranchId(marshalUtil); err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func ParseConflictMember(marshalUtil *marshalutil.MarshalUtil, optionalTargetObject ...*ConflictMember) (result *ConflictMember, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return ConflictMemberFromStorageKey(data, optionalTargetObject...)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*ConflictMember)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

func (conflictMember *ConflictMember) ConflictId() ConflictId {
	return conflictMember.conflictId
}

func (conflictMember *ConflictMember) BranchId() BranchId {
	return conflictMember.branchId
}

func (conflictMember ConflictMember) ObjectStorageKey() []byte {
	return marshalutil.New(ConflictIdLength + BranchIdLength).
		WriteBytes(conflictMember.conflictId.Bytes()).
		WriteBytes(conflictMember.branchId.Bytes()).
		Bytes()
}

func (conflictMember ConflictMember) ObjectStorageValue() []byte {
	return nil
}

func (conflictMember ConflictMember) UnmarshalObjectStorageValue([]byte) (err error, consumedBytes int) {
	return
}

func (conflictMember ConflictMember) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &ConflictMember{}

type CachedConflictMember struct {
	objectstorage.CachedObject
}

func (cachedConflictMember *CachedConflictMember) Retain() *CachedConflictMember {
	return &CachedConflictMember{cachedConflictMember.CachedObject.Retain()}
}

func (cachedConflictMember *CachedConflictMember) Unwrap() *ConflictMember {
	if untypedObject := cachedConflictMember.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*ConflictMember); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

func (cachedConflictMember *CachedConflictMember) Consume(consumer func(conflictMember *ConflictMember), forceRelease ...bool) (consumed bool) {
	return cachedConflictMember.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*ConflictMember))
	}, forceRelease...)
}

type CachedConflictMembers []*CachedConflictMember

func (cachedConflictMembers CachedConflictMembers) Consume(consumer func(conflictMember *ConflictMember)) (consumed bool) {
	for _, cachedConflictMember := range cachedConflictMembers {
		consumed = cachedConflictMember.Consume(func(output *ConflictMember) {
			consumer(output)
		}) || consumed
	}

	return
}

func (cachedConflictMembers CachedConflictMembers) Release(force ...bool) {
	for _, cachedConflictMember := range cachedConflictMembers {
		cachedConflictMember.Release(force...)
	}
}
