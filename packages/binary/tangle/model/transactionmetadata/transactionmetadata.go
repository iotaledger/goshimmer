package transactionmetadata

import (
	"sync"
	"time"

	"github.com/iotaledger/hive.go/marshalutil"
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

func FromBytes(bytes []byte) (result *TransactionMetadata, err error, consumedBytes int) {
	marshalUtil := marshalutil.New(bytes)
	result, err = Parse(marshalUtil)
	consumedBytes = marshalUtil.ReadOffset()

	return
}

func Parse(marshalUtil *marshalutil.MarshalUtil) (result *TransactionMetadata, err error) {
	if parsedObject, parseErr := marshalUtil.Parse(func(data []byte) (interface{}, error, int) {
		return StorableObjectFromKey(data)
	}); parseErr != nil {
		err = parseErr

		return
	} else {
		result = parsedObject.(*TransactionMetadata)
	}

	if _, err = marshalUtil.Parse(func(data []byte) (parseResult interface{}, parseErr error, parsedBytes int) {
		parseErr, parsedBytes = result.UnmarshalObjectStorageValue(data)

		return
	}); err != nil {
		return
	}

	return
}

func StorableObjectFromKey(key []byte) (result objectstorage.StorableObject, err error, consumedBytes int) {
	result = &TransactionMetadata{}

	marshalUtil := marshalutil.New(key)
	result.(*TransactionMetadata).transactionId, err = message.ParseId(marshalUtil)
	if err != nil {
		return
	}
	consumedBytes = marshalUtil.ReadOffset()

	return
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

func (transactionMetadata *TransactionMetadata) ObjectStorageKey() []byte {
	return transactionMetadata.transactionId.Bytes()
}

func (transactionMetadata *TransactionMetadata) ObjectStorageValue() []byte {
	return marshalutil.New().
		WriteTime(transactionMetadata.receivedTime).
		WriteTime(transactionMetadata.solidificationTime).
		WriteBool(transactionMetadata.solid).
		Bytes()
}

func (transactionMetadata *TransactionMetadata) UnmarshalObjectStorageValue(data []byte) (err error, consumedBytes int) {
	marshalUtil := marshalutil.New(data)

	if transactionMetadata.receivedTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if transactionMetadata.solidificationTime, err = marshalUtil.ReadTime(); err != nil {
		return
	}
	if transactionMetadata.solid, err = marshalUtil.ReadBool(); err != nil {
		return
	}

	consumedBytes = marshalUtil.ReadOffset()

	return
}

func (transactionMetadata *TransactionMetadata) Update(other objectstorage.StorableObject) {

}

var _ objectstorage.StorableObject = &TransactionMetadata{}
