package missingtransaction

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/hive.go/objectstorage"
)

type MissingTransaction struct {
	objectstorage.StorableObjectFlags

	transactionId message.Id
	missingSince  time.Time
}

func New(transactionId message.Id) *MissingTransaction {
	return &MissingTransaction{
		transactionId: transactionId,
		missingSince:  time.Now(),
	}
}

func StorableObjectFromKey(key []byte) (objectstorage.StorableObject, error) {
	result := &MissingTransaction{}
	copy(result.transactionId[:], key)

	return result, nil
}

func (missingTransaction *MissingTransaction) GetTransactionId() message.Id {
	return missingTransaction.transactionId
}

func (missingTransaction *MissingTransaction) GetMissingSince() time.Time {
	return missingTransaction.missingSince
}

func (missingTransaction *MissingTransaction) Update(other objectstorage.StorableObject) {
	panic("missing transactions should never be overwritten and only stored once to optimize IO")
}

func (missingTransaction *MissingTransaction) ObjectStorageKey() []byte {
	return missingTransaction.transactionId[:]
}

func (missingTransaction *MissingTransaction) ObjectStorageValue() (result []byte) {
	result, err := missingTransaction.missingSince.MarshalBinary()
	if err != nil {
		panic(err)
	}

	return
}

func (missingTransaction *MissingTransaction) UnmarshalObjectStorageValue(data []byte) (err error) {
	return missingTransaction.missingSince.UnmarshalBinary(data)
}
