package approver

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/transaction"
)

type Approver struct {
	objectstorage.StorableObjectFlags

	storageKey            []byte
	referencedTransaction transaction.Id
	approvingTransaction  transaction.Id
}

func New(referencedTransaction transaction.Id, approvingTransaction transaction.Id) *Approver {
	approver := &Approver{
		storageKey:            make([]byte, transaction.IdLength+transaction.IdLength),
		referencedTransaction: referencedTransaction,
		approvingTransaction:  approvingTransaction,
	}

	copy(approver.storageKey[:transaction.IdLength], referencedTransaction[:])
	copy(approver.storageKey[transaction.IdLength:], approvingTransaction[:])

	return approver
}

func FromStorage(id []byte) (result objectstorage.StorableObject) {
	approver := &Approver{
		storageKey: make([]byte, transaction.IdLength+transaction.IdLength),
	}
	copy(approver.referencedTransaction[:], id[:transaction.IdLength])
	copy(approver.approvingTransaction[:], id[transaction.IdLength:])
	copy(approver.storageKey, id)

	return approver
}

func (approver *Approver) GetStorageKey() []byte {
	return approver.storageKey
}

func (approver *Approver) GetApprovingTransactionId() transaction.Id {
	return approver.approvingTransaction
}

func (approver *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

func (approver *Approver) MarshalBinary() (result []byte, err error) {
	return
}

func (approver *Approver) UnmarshalBinary(data []byte) (err error) {
	return
}
