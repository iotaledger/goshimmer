package branchmanager

import (
	"sync"

	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/iotaledger/hive.go/stringify"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
)

type Conflict struct {
	objectstorage.StorableObjectFlags

	id          transaction.OutputId
	memberCount uint32

	memberCountMutex sync.RWMutex
}

func NewConflict(id transaction.OutputId) *Conflict {
	return &Conflict{
		id: id,
	}
}

func (conflict *Conflict) Id() transaction.OutputId {
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
