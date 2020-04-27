package branchmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"
)

type Conflict struct {
	objectstorage.StorableObjectFlags

	id          ConflictId
	memberCount uint32

	memberCountMutex sync.RWMutex
}

func NewConflict(id ConflictId) *Conflict {
	return &Conflict{
		id: id,
	}
}

func (conflict *Conflict) Id() ConflictId {
	return conflict.id
}

func (conflict *Conflict) MemberCount() int {
	conflict.memberCountMutex.RLock()
	defer conflict.memberCountMutex.RLock()

	return int(conflict.memberCount)
}

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

func (conflict *Conflict) Bytes() []byte {
	return marshalutil.New().
		WriteBytes(conflict.ObjectStorageKey()).
		WriteBytes(conflict.ObjectStorageValue()).
		Bytes()
}

func (conflict *Conflict) String() string {
	return stringify.Struct("Conflict",
		stringify.StructField("id", conflict.id),
		stringify.StructField("memberCount", conflict.MemberCount()),
	)
}

func (conflict *Conflict) ObjectStorageKey() []byte {
	return conflict.id.Bytes()
}

func (conflict *Conflict) ObjectStorageValue() []byte {
	return marshalutil.New(marshalutil.UINT32_SIZE).
		WriteUint32(conflict.memberCount).
		Bytes()
}

func (conflict *Conflict) UnmarshalObjectStorageValue(valueBytes []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(valueBytes)
	conflict.memberCount, err = marshalUtil.ReadUint32()
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (conflict *Conflict) Update(other objectstorage.StorableObject) {
	panic("updates are disabled - use the setters")
}

var _ objectstorage.StorableObject = &Conflict{}

type CachedConflict struct {
	objectstorage.CachedObject
}

func (cachedConflict *CachedConflict) Retain() *CachedConflict {
	return &CachedConflict{cachedConflict.CachedObject.Retain()}
}

func (cachedConflict *CachedConflict) Unwrap() *Conflict {
	if untypedObject := cachedConflict.Get(); untypedObject == nil {
		return nil
	} else {
		if typedObject := untypedObject.(*Conflict); typedObject == nil || typedObject.IsDeleted() {
			return nil
		} else {
			return typedObject
		}
	}
}

func (cachedConflict *CachedConflict) Consume(consumer func(branch *Conflict), forceRelease ...bool) (consumed bool) {
	return cachedConflict.CachedObject.Consume(func(object objectstorage.StorableObject) {
		consumer(object.(*Conflict))
	}, forceRelease...)
}
