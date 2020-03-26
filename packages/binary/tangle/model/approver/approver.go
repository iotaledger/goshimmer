package approver

import (
	"github.com/iotaledger/hive.go/objectstorage"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
)

type Approver struct {
	objectstorage.StorableObjectFlags

	storageKey            []byte
	referencedTransaction message.Id
	approvingTransaction  message.Id
}

func New(referencedTransaction message.Id, approvingTransaction message.Id) *Approver {
	approver := &Approver{
		storageKey:            make([]byte, message.IdLength+message.IdLength),
		referencedTransaction: referencedTransaction,
		approvingTransaction:  approvingTransaction,
	}

	copy(approver.storageKey[:message.IdLength], referencedTransaction[:])
	copy(approver.storageKey[message.IdLength:], approvingTransaction[:])

	return approver
}

func StorableObjectFromKey(id []byte) (result objectstorage.StorableObject, err error) {
	approver := &Approver{
		storageKey: make([]byte, message.IdLength+message.IdLength),
	}
	copy(approver.referencedTransaction[:], id[:message.IdLength])
	copy(approver.approvingTransaction[:], id[message.IdLength:])
	copy(approver.storageKey, id)

	return approver, nil
}

func (approver *Approver) ObjectStorageKey() []byte {
	return approver.storageKey
}

func (approver *Approver) GetApprovingTransactionId() message.Id {
	return approver.approvingTransaction
}

func (approver *Approver) Update(other objectstorage.StorableObject) {
	panic("approvers should never be overwritten and only stored once to optimize IO")
}

func (approver *Approver) ObjectStorageValue() (result []byte) {
	return
}

func (approver *Approver) UnmarshalObjectStorageValue(data []byte) (err error) {
	return
}
