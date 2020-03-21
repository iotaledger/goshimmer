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

func FromStorage(key []byte) objectstorage.StorableObject {
	result := &MissingTransaction{}
	copy(result.transactionId[:], key)

	return result
}

func (missingTransaction *MissingTransaction) GetTransactionId() message.Id {
	return missingTransaction.transactionId
}

func (missingTransaction *MissingTransaction) GetMissingSince() time.Time {
	return missingTransaction.missingSince
}

func (missingTransaction *MissingTransaction) GetStorageKey() []byte {
	return missingTransaction.transactionId[:]
}

func (missingTransaction *MissingTransaction) Update(other objectstorage.StorableObject) {
	panic("missing transactions should never be overwritten and only stored once to optimize IO")
}

func (missingTransaction *MissingTransaction) MarshalBinary() (result []byte, err error) {
	return missingTransaction.missingSince.MarshalBinary()
}

func (missingTransaction *MissingTransaction) UnmarshalBinary(data []byte) (err error) {
	return missingTransaction.missingSince.UnmarshalBinary(data)
}
