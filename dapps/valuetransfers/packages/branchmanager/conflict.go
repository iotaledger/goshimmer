package branchmanager

import (
	"fmt"
	"sync"

	"github.com/iotaledger/hive.go/byteutils"
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

// ConflictFromBytes unmarshals a Conflict from a sequence of bytes.
func ConflictFromBytes(bytes []byte) (result *Conflict, consumedBytes int, err error) {
	marshalUtil := marshalutil.New(bytes)
	result, err = ParseConflict(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

// ConflictFromObjectStorage is a factory method that creates a new Conflict instance from the ObjectStorage.
func ConflictFromObjectStorage(key []byte, data []byte) (result objectstorage.StorableObject, err error) {
	if result, _, err = ConflictFromBytes(byteutils.ConcatBytes(key, data)); err != nil {
		return
	}

	return
}

// ParseConflict unmarshals a Conflict using the given marshalUtil (for easier marshaling/unmarshaling).
func ParseConflict(marshalUtil *marshalutil.MarshalUtil) (result *Conflict, err error) {
	result = &Conflict{}

	if result.id, err = ParseConflictID(marshalUtil); err != nil {
		err = fmt.Errorf("failed to parse conflict ID: %w", err)
		return
	}
	if result.memberCount, err = marshalUtil.ReadUint32(); err != nil {
		err = fmt.Errorf("failed to parse memberCount of conflict: %w", err)
		return
	}

	return
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
func (conflict *Conflict) IncreaseMemberCount(optionalDelta ...int) int {
	delta := uint32(1)
	if len(optionalDelta) >= 1 {
		delta = uint32(optionalDelta[0])
	}

	conflict.memberCountMutex.Lock()
	defer conflict.memberCountMutex.Unlock()

	conflict.memberCount = conflict.memberCount + delta
	conflict.SetModified()

	return int(conflict.memberCount)
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
	conflict.SetModified()
	newMemberCount = int(conflict.memberCount)

	return
}

// Bytes returns a marshaled version of this Conflict.
func (conflict *Conflict) Bytes() []byte {
	return byteutils.ConcatBytes(conflict.ObjectStorageKey(), conflict.ObjectStorageValue())
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
	return marshalutil.New(marshalutil.Uint32Size).
		WriteUint32(uint32(conflict.MemberCount())).
		Bytes()
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
func (cachedConflict *CachedConflict) Consume(consumer func(conflict *Conflict), forceRelease ...bool) (consumed bool) {
	return cachedConflict.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Conflict))
	}, forceRelease...)
}
