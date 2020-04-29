package branchmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

// Conflict represents a
type Conflict struct {
	objectstorage.StorableObjectFlags

	id          ConflictID
	memberCount uint32

	memberCountMutex sync.RWMutex
}

// NewConflict is the constructor for new Conflicts.
func NewConflict(id ConflictID) *Conflict {
	return &Conflict{
		id: id,
	}
}

// ID returns the identifier of this Conflict.
func (conflict *Conflict) ID() ConflictID {
	return conflict.id
}

// MemberCount returns the amount of Branches that are part of this Conflict.
func (conflict *Conflict) MemberCount() int {
	conflict.memberCountMutex.RLock()
	defer conflict.memberCountMutex.RLock()

	return int(conflict.memberCount)
}

// IncreaseMemberCount offers a thread safe way to increase the MemberCount property.
func (conflict *Conflict) IncreaseMemberCount(optionalDelta ...int) (newMemberCount int) {
	delta := uint32(1)
	if len(optionalDelta) >= 1 {
		delta = uint32(optionalDelta[0])
	}

	conflict.memberCountMutex.Lock()
	defer conflict.memberCountMutex.Unlock()

	conflict.memberCount = conflict.memberCount + delta

	newMemberCount = int(conflict.memberCount)

	return
}

// DecreaseMemberCount offers a thread safe way to decrease the MemberCount property.
func (conflict *Conflict) DecreaseMemberCount(optionalDelta ...int) (newMemberCount int) {
	delta := uint32(1)
	if len(optionalDelta) >= 1 {
		delta = uint32(optionalDelta[0])
	}

	conflict.memberCountMutex.Lock()
	defer conflict.memberCountMutex.Unlock()

	conflict.memberCount = conflict.memberCount - delta

	newMemberCount = int(conflict.memberCount)

	return
}

// Bytes returns a marshaled version of this Conflict.
func (conflict *Conflict) Bytes() []byte {
	return marshalutil.New().
		WriteBytes(conflict.ObjectStorageKey()).
		WriteBytes(conflict.ObjectStorageValue()).
		Bytes()
}

// String returns a human readable version of this Conflict (for debug purposes).
func (conflict *Conflict) String() string {
	return stringify.Struct("Conflict",
		stringify.StructField("id", conflict.id),
		stringify.StructField("memberCount", conflict.MemberCount()),
	)
}

// ObjectStorageKey returns the bytes that are used a key when storing the Branch in an objectstorage.
func (conflict *Conflict) ObjectStorageKey() []byte {
	return conflict.id.Bytes()
}

// ObjectStorageValue returns the bytes that represent all remaining information (not stored in the key) of a marshaled
// Branch.
func (conflict *Conflict) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.UINT32_SIZE).
		WriteUint32(conflict.memberCount).
		Bytes()
}

// UnmarshalObjectStorageValue unmarshals the bytes that are stored in the value of the objectstorage.
func (conflict *Conflict) UnmarshalObjectStorageValue(valueBytes []byte) (consumedBytes int, err error) {
	marshalUtil := marshalutil.New(valueBytes)
	conflict.memberCount, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// Update is disabled but needs to be implemented to be compatible with the objectstorage.
func (conflict *Conflict) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &Conflict{}

// CachedConflict is a wrapper for the generic CachedObject returned by the objectstorage, that overrides the accessor
// methods, with a type-casted one.
type CachedConflict struct {
	objectstorage.CachedObject
}

// Retain marks this CachedObject to still be in use by the program.
func (cachedConflict *CachedConflict) Retain() *CachedConflict {
	return &CachedConflict{cachedConflict.CachedObject.Retain()}
}

// Unwrap is the type-casted equivalent of Get. It returns nil if the object does not exist.
func (cachedConflict *CachedConflict) Unwrap() *Conflict {
	untypedObject := cachedConflict.Get()
	if untypedObject == nil {
		return nil
	}

	typedObject := untypedObject.(*Conflict)
	if typedObject == nil || typedObject.IsDeleted() {
		return nil
	}

	return typedObject
}

// Consume unwraps the CachedObject and passes a type-casted version to the consumer (if the object is not empty - it
// exists). It automatically releases the object when the consumer finishes.
func (cachedConflict *CachedConflict) Consume(consumer func(branch *Conflict), forceRelease ...bool) (consumed bool) {
	return cachedConflict.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Conflict))
	}, forceRelease...)
}
