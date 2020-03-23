package transactionmetadata

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

type TransactionMetadata struct {
	objectstorage.StorableObjectFlags

	transactionId      message.Id
	receivedTime       time.Time
	solid              bool
	solidificationTime time.Time

	solidMutex              sync.RWMutex
	solidificationTimeMutex sync.RWMutex
}

func New(transactionId message.Id) *TransactionMetadata {
	return &TransactionMetadata{
		transactionId: transactionId,
		receivedTime:  time.Now(),
	}
}

func FromStorage(id []byte) objectstorage.StorableObject {
	result := &TransactionMetadata{}
	copy(result.transactionId[:], id)

	return result
}

func (transactionMetadata *TransactionMetadata) IsSolid() (result bool) {
	transactionMetadata.solidMutex.RLock()
	result = transactionMetadata.solid
	transactionMetadata.solidMutex.RUnlock()

	return
}

func (transactionMetadata *TransactionMetadata) SetSolid(solid bool) (modified bool) {
	transactionMetadata.solidMutex.RLock()
	if transactionMetadata.solid != solid {
		transactionMetadata.solidMutex.RUnlock()

		transactionMetadata.solidMutex.Lock()
		if transactionMetadata.solid != solid {
			transactionMetadata.solid = solid
			if solid {
				transactionMetadata.solidificationTimeMutex.Lock()
				transactionMetadata.solidificationTime = time.Now()
				transactionMetadata.solidificationTimeMutex.Unlock()
			}

			transactionMetadata.SetModified()

			modified = true
		}
		transactionMetadata.solidMutex.Unlock()

	} else {
		transactionMetadata.solidMutex.RUnlock()
	}

	return
}

func (transactionMetadata *TransactionMetadata) GetSoldificationTime() time.Time {
	transactionMetadata.solidificationTimeMutex.RLock()
	defer transactionMetadata.solidificationTimeMutex.RUnlock()

	return transactionMetadata.solidificationTime
}

func (transactionMetadata *TransactionMetadata) GetStorageKey() []byte {
	return transactionMetadata.transactionId[:]
}

func (transactionMetadata *TransactionMetadata) Update(other objectstorage.StorableObject) {

}

func (transactionMetadata *TransactionMetadata) MarshalBinary() ([]byte, error) {
	return (&Proto{
		receivedTime:       transactionMetadata.receivedTime,
		solidificationTime: transactionMetadata.solidificationTime,
		solid:              transactionMetadata.solid,
	}).ToBytes(), nil
}

func (transactionMetadata *TransactionMetadata) UnmarshalBinary(data []byte) (err error) {
	proto, err := ProtoFromBytes(data)
	if err != nil {
		return
	}

	transactionMetadata.receivedTime = proto.receivedTime
	transactionMetadata.solidificationTime = proto.solidificationTime
	transactionMetadata.solid = proto.solid

	return
}
