package valuetangle

import (
	"sync"

	"github.com/iotaledger/hive.go/objectstorage"
	"golang.org/x/crypto/blake2b"
)

type Transfer struct {
	objectstorage.StorableObjectFlags

	id    *TransferId
	bytes []byte

	idMutex    sync.RWMutex
	bytesMutex sync.RWMutex
}

func NewTransfer() *Transfer {
	return &Transfer{}
}

func FromStorage(key []byte) *Transfer {
	id := NewTransferId(key)

	return &Transfer{
		id: &id,
	}
}

func (transfer *Transfer) GetId() TransferId {
	// acquire lock for reading id
	transfer.idMutex.RLock()

	// return if id has been calculated already
	if transfer.id != nil {
		defer transfer.idMutex.RUnlock()

		return *transfer.id
	}

	// switch to write lock
	transfer.idMutex.RUnlock()
	transfer.idMutex.Lock()
	defer transfer.idMutex.Unlock()

	// return if id has been calculated in the mean time
	if transfer.id != nil {
		return *transfer.id
	}

	// otherwise calculate the PayloadId
	transfer.id = transfer.calculateId()

	return *transfer.id
}

func (transfer *Transfer) calculateId() *TransferId {
	bytes, _ := transfer.MarshalBinary()

	id := blake2b.Sum256(bytes)

	result := NewTransferId(id[:])

	return &result
}

func (transfer *Transfer) Update(other objectstorage.StorableObject) {
	panic("implement me")
}

func (transfer *Transfer) GetStorageKey() []byte {
	panic("implement me")
}

func (transfer *Transfer) MarshalBinary() ([]byte, error) {
	return nil, nil
}

func (transfer *Transfer) UnmarshalBinary(data []byte) error {
	panic("implement me")
}

// define contracts (ensure that the struct fulfills the corresponding interfaces
var _ objectstorage.StorableObject = &Transfer{}
